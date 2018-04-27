/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"
	"k8s.io/kops/util/pkg/vfs"

	apis_etcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/pkg/backup"
	"kope.io/etcd-manager/pkg/controller"
	"kope.io/etcd-manager/pkg/etcd"
	"kope.io/etcd-manager/pkg/locking"
	"kope.io/etcd-manager/pkg/privateapi"
	"kope.io/etcd-manager/pkg/privateapi/discovery"
	vfsdiscovery "kope.io/etcd-manager/pkg/privateapi/discovery/vfs"
)

type stringSliceFlag []string

func (v *stringSliceFlag) String() string {
	return fmt.Sprintf("%v", *v)
}

func (v *stringSliceFlag) Set(value string) error {
	*v = append(*v, value)
	return nil
}

func main() {
	flag.Set("logtostderr", "true")

	//flag.BoolVar(&volumes.Containerized, "containerized", volumes.Containerized, "set if we are running containerized")

	var o EtcdManagerOptions
	o.InitDefaults()

	flag.StringVar(&o.Address, "address", o.Address, "local address to use")
	flag.IntVar(&o.PeerPort, "peer-port", o.PeerPort, "peer-port to use")
	flag.IntVar(&o.GrpcPort, "grpc-port", o.GrpcPort, "grpc-port to use")
	flag.StringVar(&o.ClientUrls, "client-urls", o.ClientUrls, "client-urls to use for normal operation")
	flag.StringVar(&o.QuarantineClientUrls, "quarantine-client-urls", o.QuarantineClientUrls, "client-urls to use when etcd should be quarantined e.g. when offline")
	flag.StringVar(&o.ClusterName, "cluster-name", o.ClusterName, "name of cluster")
	flag.StringVar(&o.BackupStorePath, "backup-store", o.BackupStorePath, "backup store location")
	flag.StringVar(&o.DataDir, "data-dir", o.DataDir, "directory for storing etcd data")

	flag.StringVar(&o.VolumeProviderID, "volume-provider", o.VolumeProviderID, "provider for volumes")

	var volumeTags stringSliceFlag
	flag.Var(&volumeTags, "volume-tag", "tag which volume is required to have")

	flag.Parse()

	o.VolumeTags = volumeTags

	fmt.Printf("etcd-manager\n")

	err := RunEtcdManager(&o)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
		os.Exit(1)
	} else {
		os.Exit(0)
	}
}

// EtcdManagerOptions holds the flag options for running etcd-manager
type EtcdManagerOptions struct {
	Address              string
	VolumeProviderID     string
	PeerPort             int
	GrpcPort             int
	ClientUrls           string
	QuarantineClientUrls string
	ClusterName          string
	BackupStorePath      string
	DataDir              string
	VolumeTags           []string
}

// InitDefaults populates the default flag values
func (o *EtcdManagerOptions) InitDefaults() {
	o.Address = "127.0.0.1"
	o.PeerPort = 2380
	o.ClientUrls = "http://127.0.0.1:4001"
	o.QuarantineClientUrls = "http://127.0.0.1:8001"
	o.GrpcPort = 8000
	// o.BackupStorePath = "/backups"
	// o.DataDir = "/data"
}

// RunEtcdManager runs the etcd-manager, returning only we should exit.
func RunEtcdManager(o *EtcdManagerOptions) error {
	if o.ClusterName == "" {
		return fmt.Errorf("cluster-name is required")
	}

	if o.BackupStorePath == "" {
		return fmt.Errorf("backup-store is required")
	}

	var discoveryProvider discovery.Interface
	var myPeerId privateapi.PeerId

	if o.VolumeProviderID != "" {
		// var volumeProvider volumes.Volumes

		switch o.VolumeProviderID {
		case "aws":
			// awsVolumeProvider, err := aws.NewAWSVolumes(o.VolumeTags, "%s:"+strconv.Itoa(o.GrpcPort))
			// if err != nil {
			// 	fmt.Fprintf(os.Stderr, "%v\n", err)
			// 	os.Exit(1)
			// }

			// volumeProvider = awsVolumeProvider
			// discoveryProvider = awsVolumeProvider

		default:
			fmt.Fprintf(os.Stderr, "unknown volume-provider %q\n", o.VolumeProviderID)
			os.Exit(1)
		}

		// boot := &volumes.Boot{}
		// boot.Init(volumeProvider)

		// glog.Infof("Mounting available etcd volumes matching tags %v", o.VolumeTags)
		// volumes := boot.WaitForVolumes()
		// if len(volumes) == 0 {
		// 	return fmt.Errorf("no volumes were mounted")
		// }

		// if len(volumes) != 1 {
		// 	return fmt.Errorf("multiple volumes were mounted: %v", volumes)
		// }

		// o.DataDir = volumes[0].Mountpoint
		// myPeerId = privateapi.PeerId(volumes[0].ID)

		// glog.Infof("Setting data dir to %s", o.DataDir)
	}

	if err := os.MkdirAll(o.DataDir, 0755); err != nil {
		return fmt.Errorf("error doing mkdirs on base directory %s: %v", o.DataDir, err)
	}

	if myPeerId == "" {
		uniqueID, err := privateapi.PersistentPeerId(o.DataDir)
		if err != nil {
			return fmt.Errorf("error getting persistent peer id: %v", err)
		}
		myPeerId = uniqueID
	}

	grpcEndpoint := fmt.Sprintf("%s:%d", o.Address, o.GrpcPort)

	if discoveryProvider == nil {
		discoMe := discovery.Node{
			ID: string(myPeerId),
		}

		discoMe.Endpoints = append(discoMe.Endpoints, discovery.NodeEndpoint{
			Endpoint: grpcEndpoint,
		})

		glog.Warningf("Using fake discovery manager")
		p, err := vfs.Context.BuildVfsPath("file:///tmp/discovery")
		if err != nil {
			return fmt.Errorf("error parsing vfs path: %v", err)
		}
		vfsDiscovery, err := vfsdiscovery.NewVFSDiscovery(p, discoMe)
		if err != nil {
			return fmt.Errorf("error building discovery: %v", err)
		}
		discoveryProvider = vfsDiscovery
	}

	ctx := context.TODO()

	myInfo := privateapi.PeerInfo{
		Id:        string(myPeerId),
		Endpoints: []string{grpcEndpoint},
	}
	peerServer, err := privateapi.NewServer(ctx, myInfo, discoveryProvider)
	if err != nil {
		return fmt.Errorf("error building server: %v", err)
	}

	var peerUrls []string
	peerUrls = append(peerUrls, fmt.Sprintf("http://%s:%d", o.Address, o.PeerPort))

	etcdNodeInfo := &apis_etcd.EtcdNode{
		Name:                  string(myPeerId),
		ClientUrls:            strings.Split(o.ClientUrls, ","),
		QuarantinedClientUrls: strings.Split(o.QuarantineClientUrls, ","),
		PeerUrls:              peerUrls,
	}

	backupStore, err := backup.NewStore(o.BackupStorePath)
	if err != nil {
		return fmt.Errorf("error initializing backup store: %v", err)
	}

	etcdServer, err := etcd.NewEtcdServer(o.DataDir, o.ClusterName, etcdNodeInfo, peerServer)
	if err != nil {
		return fmt.Errorf("error initializing etcd server: %v", err)
	}
	go etcdServer.Run(ctx)

	var leaderLock locking.Lock // nil
	c, err := controller.NewEtcdController(leaderLock, backupStore, o.ClusterName, peerServer)
	if err != nil {
		return fmt.Errorf("error building etcd controller: %v", err)
	}
	go func() {
		time.Sleep(2 * time.Second) // Gives a bit of time for discovery to run first
		c.Run(ctx)
	}()

	if err := peerServer.ListenAndServe(ctx, grpcEndpoint); err != nil {
		if ctx.Err() == nil {
			return fmt.Errorf("error creating private API server: %v", err)
		}
	}

	return nil
}
