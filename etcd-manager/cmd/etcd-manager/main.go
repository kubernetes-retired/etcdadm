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
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"k8s.io/kops/util/pkg/vfs"
	apis_etcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/pkg/backup"
	"kope.io/etcd-manager/pkg/commands"
	"kope.io/etcd-manager/pkg/controller"
	"kope.io/etcd-manager/pkg/etcd"
	"kope.io/etcd-manager/pkg/legacy"
	"kope.io/etcd-manager/pkg/locking"
	"kope.io/etcd-manager/pkg/pki"
	"kope.io/etcd-manager/pkg/privateapi"
	"kope.io/etcd-manager/pkg/privateapi/discovery"
	vfsdiscovery "kope.io/etcd-manager/pkg/privateapi/discovery/vfs"
	"kope.io/etcd-manager/pkg/tlsconfig"
	"kope.io/etcd-manager/pkg/volumes"
	"kope.io/etcd-manager/pkg/volumes/aws"
	"kope.io/etcd-manager/pkg/volumes/gce"
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

	flag.BoolVar(&volumes.Containerized, "containerized", volumes.Containerized, "set if we are running containerized")

	var o EtcdManagerOptions
	o.InitDefaults()

	flag.StringVar(&o.Address, "address", o.Address, "local address to use")
	flag.StringVar(&o.PeerUrls, "peer-urls", o.PeerUrls, "peer-urls to use")
	flag.IntVar(&o.GrpcPort, "grpc-port", o.GrpcPort, "grpc-port to use")
	flag.StringVar(&o.ClientUrls, "client-urls", o.ClientUrls, "client-urls to use for normal operation")
	flag.StringVar(&o.QuarantineClientUrls, "quarantine-client-urls", o.QuarantineClientUrls, "client-urls to use when etcd should be quarantined e.g. when offline")
	flag.StringVar(&o.ClusterName, "cluster-name", o.ClusterName, "name of cluster")
	flag.StringVar(&o.BackupStorePath, "backup-store", o.BackupStorePath, "backup store location")
	flag.StringVar(&o.BackupInterval, "backup-interval", o.BackupInterval, "interval for periodic backups")
	flag.StringVar(&o.DataDir, "data-dir", o.DataDir, "directory for storing etcd data")

	flag.StringVar(&o.PKIDir, "pki-dir", o.PKIDir, "directory for PKI keys")

	flag.StringVar(&o.VolumeProviderID, "volume-provider", o.VolumeProviderID, "provider for volumes")

	flag.StringVar(&o.NameTag, "volume-name-tag", o.NameTag, "tag which is used for extract node id from volume")

	flag.StringVar(&o.DNSSuffix, "dns-suffix", o.DNSSuffix, "suffix which is added to member names when configuring internal DNS")

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

func expandUrls(urls string, address string, name string) []string {
	var expanded []string
	for _, s := range strings.Split(urls, ",") {
		s = strings.Replace(s, "__address__", address, -1)
		s = strings.Replace(s, "__name__", name, -1)
		expanded = append(expanded, s)
	}
	return expanded
}

// EtcdManagerOptions holds the flag options for running etcd-manager
type EtcdManagerOptions struct {
	PKIDir               string
	Address              string
	VolumeProviderID     string
	PeerUrls             string
	GrpcPort             int
	ClientUrls           string
	QuarantineClientUrls string
	ClusterName          string
	ListenAddress        string
	BackupStorePath      string
	DataDir              string
	VolumeTags           []string
	NameTag              string
	BackupInterval       string

	// DNSSuffix is added to etcd member names and we then use internal client id discovery
	DNSSuffix string

	// ControlRefreshInterval is the interval with which we refresh the control store
	// (we also refresh on leadership changes and on boot)
	ControlRefreshInterval time.Duration
}

// InitDefaults populates the default flag values
func (o *EtcdManagerOptions) InitDefaults() {
	//o.Address = "127.0.0.1"

	o.BackupInterval = "15m"

	o.ClientUrls = "http://__address__:4001"
	o.QuarantineClientUrls = "http://__address__:8001"
	o.PeerUrls = "http://__address__:2380"

	o.GrpcPort = 8000

	// We effectively only refresh on leadership changes
	o.ControlRefreshInterval = 10 * 365 * 24 * time.Hour

	// o.BackupStorePath = "/backups"
	// o.DataDir = "/data"

	// o.PKIDir = "/etc/kubernetes/pki/etcd-manager"
}

// RunEtcdManager runs the etcd-manager, returning only we should exit.
func RunEtcdManager(o *EtcdManagerOptions) error {
	if o.ClusterName == "" {
		return fmt.Errorf("cluster-name is required")
	}

	if o.BackupStorePath == "" {
		return fmt.Errorf("backup-store is required")
	}

	backupInterval, err := time.ParseDuration(o.BackupInterval)
	if err != nil {
		return fmt.Errorf("invalid backup-interval duration %q", o.BackupInterval)
	}

	// We could make this configurable?
	o.ListenAddress = "0.0.0.0"

	var discoveryProvider discovery.Interface
	var myPeerId privateapi.PeerId

	if o.VolumeProviderID != "" {
		var volumeProvider volumes.Volumes

		switch o.VolumeProviderID {
		case "aws":
			awsVolumeProvider, err := aws.NewAWSVolumes(o.ClusterName, o.VolumeTags, o.NameTag, "%s:"+strconv.Itoa(o.GrpcPort))
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}

			volumeProvider = awsVolumeProvider
			discoveryProvider = awsVolumeProvider

		case "gce":
			gceVolumeProvider, err := gce.NewGCEVolumes(o.ClusterName, o.VolumeTags, o.NameTag, "%s:"+strconv.Itoa(o.GrpcPort))
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}

			volumeProvider = gceVolumeProvider
			discoveryProvider = gceVolumeProvider

		default:
			fmt.Fprintf(os.Stderr, "unknown volume-provider %q\n", o.VolumeProviderID)
			os.Exit(1)
		}

		boot := &volumes.Boot{}
		boot.Init(volumeProvider)

		glog.Infof("Mounting available etcd volumes matching tags %v; nameTag=%s", o.VolumeTags, o.NameTag)
		volumes := boot.WaitForVolumes()
		if len(volumes) == 0 {
			return fmt.Errorf("no volumes were mounted")
		}

		if len(volumes) != 1 {
			return fmt.Errorf("multiple volumes were mounted: %v", volumes)
		}

		address, err := volumeProvider.MyIP()
		if err != nil {
			return err
		}
		o.Address = address
		glog.Infof("discovered IP address: %s", o.Address)

		o.DataDir = volumes[0].Mountpoint
		myPeerId = privateapi.PeerId(volumes[0].EtcdName)

		glog.Infof("Setting data dir to %s", o.DataDir)
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

	if o.Address == "" {
		glog.Infof("using 127.0.0.1 for address")
		o.Address = "127.0.0.1"
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

	var grpcServerTLS *tls.Config
	var grpcClientTLS *tls.Config
	if o.PKIDir != "" {
		store := pki.NewFSStore(o.PKIDir)
		keypairs := &pki.Keypairs{Store: store}

		grpcServerTLS, err = tlsconfig.GRPCServerConfig(keypairs, string(myPeerId))
		if err != nil {
			return err
		}

		grpcClientTLS, err = tlsconfig.GRPCClientConfig(keypairs, string(myPeerId))
		if err != nil {
			return err
		}
	}

	ctx := context.TODO()

	myInfo := privateapi.PeerInfo{
		Id:        string(myPeerId),
		Endpoints: []string{grpcEndpoint},
	}
	peerServer, err := privateapi.NewServer(ctx, myInfo, grpcServerTLS, discoveryProvider, grpcClientTLS)
	if err != nil {
		return fmt.Errorf("error building server: %v", err)
	}

	name := string(myPeerId)
	if o.DNSSuffix != "" {
		if !strings.HasSuffix(name, ".") {
			name += "."
		}
		name += strings.TrimPrefix(o.DNSSuffix, ".")
	}

	etcdNodeInfo := &apis_etcd.EtcdNode{
		Name:                  string(myPeerId),
		ClientUrls:            expandUrls(o.ClientUrls, o.Address, name),
		QuarantinedClientUrls: expandUrls(o.QuarantineClientUrls, o.Address, name),
		PeerUrls:              expandUrls(o.PeerUrls, o.Address, name),
	}

	backupStore, err := backup.NewStore(o.BackupStorePath)
	if err != nil {
		return fmt.Errorf("error initializing backup store: %v", err)
	}

	commandStore, err := commands.NewStore(o.BackupStorePath)
	if err != nil {
		glog.Fatalf("error initializing command store: %v", err)
	}

	if _, err := legacy.ScanForExisting(o.DataDir, commandStore); err != nil {
		glog.Fatalf("error performing scan for legacy data: %v", err)
	}

	etcdServer, err := etcd.NewEtcdServer(o.DataDir, o.ClusterName, o.ListenAddress, etcdNodeInfo, peerServer)
	if err != nil {
		return fmt.Errorf("error initializing etcd server: %v", err)
	}
	go etcdServer.Run(ctx)

	var leaderLock locking.Lock // nil
	c, err := controller.NewEtcdController(leaderLock, backupStore, backupInterval, commandStore, o.ControlRefreshInterval, o.ClusterName, o.DNSSuffix, peerServer)
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
