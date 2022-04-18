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
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"k8s.io/klog/v2"
	"k8s.io/kops/util/pkg/vfs"
	"sigs.k8s.io/etcdadm/apis"
	apis_etcd "sigs.k8s.io/etcdadm/etcd-manager/pkg/apis/etcd"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/backup"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/commands"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/controller"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/etcd"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/hosts"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/legacy"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/locking"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/metrics"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/pki"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/privateapi"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/privateapi/discovery"
	vfsdiscovery "sigs.k8s.io/etcdadm/etcd-manager/pkg/privateapi/discovery/vfs"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/tlsconfig"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/urls"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/volumes"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/volumes/alicloud"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/volumes/aws"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/volumes/azure"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/volumes/do"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/volumes/external"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/volumes/gce"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/volumes/openstack"
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
	klog.InitFlags(nil)

	flag.BoolVar(&volumes.Containerized, "containerized", volumes.Containerized, "set if we are running containerized")

	var o EtcdManagerOptions
	o.InitDefaults()

	flag.StringVar(&o.ListenAddress, "etcd-address", o.ListenAddress, "address on which etcd listens")
	flag.StringVar(&o.Address, "address", o.Address, "local address to use")
	flag.StringVar(&o.PeerUrls, "peer-urls", o.PeerUrls, "peer-urls to use")
	flag.IntVar(&o.GrpcPort, "grpc-port", o.GrpcPort, "grpc-port to use")
	flag.IntVar(&o.EtcdManagerMetricsPort, "etcd-manager-metrics-port", o.EtcdManagerMetricsPort, "etcd-manager prometheus metrics port")
	flag.StringVar(&o.ListenMetricsURLs, "listen-metrics-urls", o.ListenMetricsURLs, "listen-metrics-urls configure etcd dedicated metrics URL endpoints")
	flag.StringVar(&o.ClientUrls, "client-urls", o.ClientUrls, "client-urls to use for normal operation")
	flag.StringVar(&o.QuarantineClientUrls, "quarantine-client-urls", o.QuarantineClientUrls, "client-urls to use when etcd should be quarantined e.g. when offline")
	flag.StringVar(&o.ClusterName, "cluster-name", o.ClusterName, "name of cluster")
	flag.StringVar(&o.BackupStorePath, "backup-store", o.BackupStorePath, "backup store location")
	flag.StringVar(&o.BackupInterval, "backup-interval", o.BackupInterval, "interval for periodic backups")
	flag.StringVar(&o.DiscoveryPollInterval, "discovery-poll-interval", o.DiscoveryPollInterval, "interval for discovery poll")
	flag.StringVar(&o.DataDir, "data-dir", o.DataDir, "directory for storing etcd data")

	flag.StringVar(&o.PKIDir, "pki-dir", o.PKIDir, "directory for PKI keys")
	flag.BoolVar(&o.Insecure, "insecure", o.Insecure, "allow use of non-secure connections for etcd-manager")
	flag.BoolVar(&o.EtcdInsecure, "etcd-insecure", o.EtcdInsecure, "allow use of non-secure connections for etcd itself")

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
	PKIDir                string
	Address               string
	VolumeProviderID      string
	PeerUrls              string
	GrpcPort              int
	ClientUrls            string
	QuarantineClientUrls  string
	ClusterName           string
	ListenAddress         string
	BackupStorePath       string
	DataDir               string
	InitSystem            apis.InitSystem
	VolumeTags            []string
	NameTag               string
	BackupInterval        string
	DiscoveryPollInterval string

	// DNSSuffix is added to etcd member names and we then use internal client id discovery
	DNSSuffix string

	// ControlRefreshInterval is the interval with which we refresh the control store
	// (we also refresh on leadership changes and on boot)
	ControlRefreshInterval time.Duration

	// We have an explicit option for insecure configuration for grpc
	Insecure bool

	// We have an explicit option for insecure configuration for etcd
	EtcdInsecure bool

	// ListenMetricsURLs allows configuration of the special etcd metrics urls
	ListenMetricsURLs string

	// EtcdManagerMetricsPort allows exposing statistics from etcd-manager
	EtcdManagerMetricsPort int
}

// InitDefaults populates the default flag values
func (o *EtcdManagerOptions) InitDefaults() {
	//o.Address = "127.0.0.1"

	o.BackupInterval = "15m"
	o.DiscoveryPollInterval = "60s"

	o.ClientUrls = "https://__address__:4001"
	o.QuarantineClientUrls = "https://__address__:8001"
	o.PeerUrls = "https://__address__:2380"

	o.GrpcPort = 8000

	o.ListenAddress = "0.0.0.0"

	// We effectively only refresh on leadership changes
	o.ControlRefreshInterval = 10 * 365 * 24 * time.Hour

	// o.BackupStorePath = "/backups"
	// o.DataDir = "/data"

	o.PKIDir = "/etc/kubernetes/pki/etcd-manager"

	o.Insecure = false
	o.EtcdInsecure = false
	o.EtcdManagerMetricsPort = 0
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

	dnsProvider := &hosts.Provider{
		Key: "etcd-manager[" + o.ClusterName + "]",
	}

	var discoveryProvider discovery.Interface
	var myPeerId privateapi.PeerId

	// start etcd-manager metrics if the etcd manager metrics port is defined
	if o.EtcdManagerMetricsPort != 0 {
		go metrics.RegisterMetrics(o.EtcdManagerMetricsPort, o.VolumeProviderID)
	}

	if o.VolumeProviderID != "" {
		var volumeProvider volumes.Volumes

		switch o.VolumeProviderID {
		case "aws":
			awsVolumeProvider, err := aws.NewAWSVolumes(o.ClusterName, o.VolumeTags, o.NameTag)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}

			volumeProvider = awsVolumeProvider
			discoveryProvider = awsVolumeProvider

		case "gce":
			gceVolumeProvider, err := gce.NewGCEVolumes(o.ClusterName, o.VolumeTags, o.NameTag)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}

			volumeProvider = gceVolumeProvider
			discoveryProvider = gceVolumeProvider

		case "openstack":
			osVolumeProvider, err := openstack.NewOpenstackVolumes(o.ClusterName, o.VolumeTags, o.NameTag)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}

			volumeProvider = osVolumeProvider
			discoveryProvider = osVolumeProvider

		case "do":
			doVolumeProvider, err := do.NewDOVolumes(o.ClusterName, o.VolumeTags, o.NameTag)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}

			volumeProvider = doVolumeProvider
			discoveryProvider = doVolumeProvider

		case "alicloud":
			alicloudVolumeProvider, err := alicloud.NewAlicloudVolumes(o.ClusterName, o.VolumeTags, o.NameTag)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}

			volumeProvider = alicloudVolumeProvider
			discoveryProvider = alicloudVolumeProvider

		case "azure":
			azureVolumeProvider, err := azure.NewAzureVolumes(o.ClusterName, o.VolumeTags, o.NameTag)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}

			volumeProvider = azureVolumeProvider
			discoveryProvider = azureVolumeProvider

		case "external":
			volumeDir := volumes.PathFor("/mnt/disks")
			externalVolumeProvider, err := external.NewExternalVolumes(o.ClusterName, volumeDir, o.VolumeTags)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
			volumeProvider = externalVolumeProvider

			// TODO: Allow this to be customized
			seedDir := volumes.PathFor("/etc/kubernetes/etcd-manager/seeds")
			discoveryProvider = external.NewExternalDiscovery(seedDir, externalVolumeProvider)

		default:
			fmt.Fprintf(os.Stderr, "unknown volume-provider %q\n", o.VolumeProviderID)
			os.Exit(1)
		}

		boot := &volumes.Boot{}
		boot.Init(volumeProvider)

		klog.Infof("Mounting available etcd volumes matching tags %v; nameTag=%s", o.VolumeTags, o.NameTag)
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
		klog.Infof("discovered IP address: %s", o.Address)

		o.DataDir = volumes[0].Mountpoint
		myPeerId = privateapi.PeerId(volumes[0].EtcdName)

		klog.Infof("Setting data dir to %s", o.DataDir)
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
		klog.Infof("using 127.0.0.1 for address")
		o.Address = "127.0.0.1"
	}

	grpcEndpoint := net.JoinHostPort(o.Address, strconv.Itoa(o.GrpcPort))

	if discoveryProvider == nil {
		discoMe := discovery.Node{
			ID: string(myPeerId),
		}

		discoMe.Endpoints = append(discoMe.Endpoints, discovery.NodeEndpoint{
			IP:   o.Address,
			Port: o.GrpcPort,
		})

		klog.Warningf("Using fake discovery manager")
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

	var etcdClientsCA *pki.CA
	var etcdPeersCA *pki.CA
	if !o.Insecure {
		if o.PKIDir == "" {
			return fmt.Errorf("pki-dir is required for secure configurations")
		}

		store := pki.NewFSStore(o.PKIDir)
		ca, err := store.LoadCA("etcd-manager-ca")
		if err != nil {
			return err
		}

		keypairs := pki.NewKeypairs(store, ca)

		grpcServerTLS, err = tlsconfig.GRPCServerConfig(keypairs, string(myPeerId))
		if err != nil {
			return err
		}

		grpcClientTLS, err = tlsconfig.GRPCClientConfig(keypairs, string(myPeerId))
		if err != nil {
			return err
		}
	}

	if !o.EtcdInsecure {
		if o.PKIDir == "" {
			return fmt.Errorf("pki-dir is required for secure configurations")
		}

		store := pki.NewFSStore(o.PKIDir)

		etcdPeersCA, err = store.LoadCA("etcd-peers-ca")
		if err != nil {
			return fmt.Errorf("error loading etcd-peers-ca keypair: %v", err)
		}

		etcdClientsCA, err = store.LoadCA("etcd-clients-ca")
		if err != nil {
			return fmt.Errorf("error loading etcd-clients-ca keypair: %v", err)
		}
	}

	ctx := context.TODO()

	myInfo := privateapi.PeerInfo{
		Id:        string(myPeerId),
		Endpoints: []string{grpcEndpoint},
	}
	discoveryPollInterval, err := time.ParseDuration(o.DiscoveryPollInterval)
	if err != nil {
		return fmt.Errorf("invalid discovery-poll-interval duration %q", o.DiscoveryPollInterval)
	}
	peerServer, err := privateapi.NewServer(ctx, myInfo, grpcServerTLS, discoveryProvider, o.GrpcPort, dnsProvider, o.DNSSuffix, grpcClientTLS, discoveryPollInterval)
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

	if o.EtcdInsecure {
		etcdNodeInfo.ClientUrls = urls.RewriteScheme(etcdNodeInfo.ClientUrls, "https://", "http://")
		etcdNodeInfo.QuarantinedClientUrls = urls.RewriteScheme(etcdNodeInfo.QuarantinedClientUrls, "https://", "http://")
		etcdNodeInfo.PeerUrls = urls.RewriteScheme(etcdNodeInfo.PeerUrls, "https://", "http://")
	}

	backupStore, err := backup.NewStore(o.BackupStorePath)
	if err != nil {
		return fmt.Errorf("error initializing backup store: %v", err)
	}

	commandStore, err := commands.NewStore(o.BackupStorePath)
	if err != nil {
		klog.Fatalf("error initializing command store: %v", err)
	}

	if _, err := legacy.ScanForExisting(o.DataDir, commandStore); err != nil {
		klog.Fatalf("error performing scan for legacy data: %v", err)
	}

	listenMetricsURLs := expandUrls(o.ListenMetricsURLs, o.Address, name)
	peerClientIPs := []net.IP{}
	if ip := net.ParseIP(o.Address); ip == nil {
		return fmt.Errorf("unable to parse address %q: %w", o.Address, err)
	} else {
		peerClientIPs = append(peerClientIPs, ip)
	}
	klog.Infof("peerClientIPs: %v", peerClientIPs)
	etcdServer, err := etcd.NewEtcdServer(o.InitSystem, o.DataDir, o.ClusterName, o.ListenAddress, listenMetricsURLs, etcdNodeInfo, peerServer, dnsProvider, etcdClientsCA, etcdPeersCA, peerClientIPs)
	if err != nil {
		return fmt.Errorf("error initializing etcd server: %v", err)
	}
	go etcdServer.Run(ctx)

	var leaderLock locking.Lock // nil
	c, err := controller.NewEtcdController(leaderLock, backupStore, backupInterval, commandStore, o.ControlRefreshInterval, o.ClusterName, o.DNSSuffix, peerServer, etcdClientsCA, o.EtcdInsecure)
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
