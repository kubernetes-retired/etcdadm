/*
Copyright 2020 The Kubernetes Authors.

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

package harness

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"k8s.io/klog/v2"
	"k8s.io/kops/util/pkg/vfs"

	apis_etcd "sigs.k8s.io/etcdadm/etcd-manager/pkg/apis/etcd"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/backup"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/commands"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/controller"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/dns"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/etcd"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/etcdclient"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/locking"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/pki"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/privateapi"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/privateapi/discovery"
	vfsdiscovery "sigs.k8s.io/etcdadm/etcd-manager/pkg/privateapi/discovery/vfs"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/tlsconfig"
)

type TestHarnessNode struct {
	TestHarness *TestHarness
	Address     string
	NodeDir     string
	EtcdVersion string

	InsecureMode bool

	ctx       context.Context
	ctxCancel context.CancelFunc

	ClientURL      string
	etcdServer     *etcd.EtcdServer
	etcdController *controller.EtcdController

	etcdClientTLSConfig *tls.Config

	ListenMetricsURLs []string
}

func (n *TestHarnessNode) Init() error {
	if err := os.MkdirAll(n.NodeDir, 0755); err != nil {
		return fmt.Errorf("error creating node directory %q: %v", n.NodeDir, err)
	}

	uniqueID, err := privateapi.PersistentPeerId(n.NodeDir)
	if err != nil {
		return fmt.Errorf("error getting persistent peer id: %v", err)
	}

	if !n.InsecureMode && n.TestHarness.etcdClientsCA != nil {
		keypairs := pki.NewKeypairs(pki.NewInMemoryStore(), n.TestHarness.etcdClientsCA)

		c, err := etcd.BuildTLSClientConfig(keypairs, string(uniqueID))
		if err != nil {
			return fmt.Errorf("error building etcd-client TLS config: %v", err)
		}
		n.etcdClientTLSConfig = c
	} else {
		n.etcdClientTLSConfig = nil
	}

	if n.InsecureMode {
		n.ClientURL = "http://" + n.Address + ":4001"
	} else {
		n.ClientURL = "https://" + n.Address + ":4001"
	}

	n.ListenMetricsURLs = []string{"https://" + n.Address + ":8080"}

	return nil
}

func (n *TestHarnessNode) Run() {
	t := n.TestHarness.T

	n.ctx, n.ctxCancel = context.WithCancel(n.TestHarness.Context)

	address := n.Address

	klog.Infof("Starting node %q", address)

	uniqueID, err := privateapi.PersistentPeerId(n.NodeDir)
	if err != nil {
		t.Fatalf("error getting persistent peer id: %v", err)
	}

	grpcPort := 8000
	grpcEndpoint := fmt.Sprintf("%s:%d", address, grpcPort)

	discoMe := discovery.Node{
		ID: string(uniqueID),
	}
	discoMe.Endpoints = append(discoMe.Endpoints, discovery.NodeEndpoint{
		IP: address,
	})
	p, err := vfs.Context.BuildVfsPath(n.TestHarness.DiscoveryStoreDir)
	if err != nil {
		klog.Fatalf("error parsing discovery path %q: %v", n.TestHarness.DiscoveryStoreDir, err)
	}
	disco, err := vfsdiscovery.NewVFSDiscovery(p, discoMe)
	if err != nil {
		klog.Fatalf("error building discovery: %v", err)
	}

	var serverTLSConfig *tls.Config
	var clientTLSConfig *tls.Config
	if !n.InsecureMode {
		store := pki.NewFSStore(filepath.Join(n.NodeDir, "pki"))
		keypairs := pki.NewKeypairs(store, n.TestHarness.grpcCA)

		serverTLSConfig, err = tlsconfig.GRPCServerConfig(keypairs, string(uniqueID))
		if err != nil {
			t.Fatalf("error building grpc-server TLS config: %v", err)
		}

		clientTLSConfig, err = tlsconfig.GRPCClientConfig(keypairs, string(uniqueID))
		if err != nil {
			t.Fatalf("error building grpc-client TLS config: %v", err)
		}
	}

	dnsProvider := &MockDNSProvider{}
	dnsSuffix := "mock.local"

	myInfo := privateapi.PeerInfo{
		Id:        string(uniqueID),
		Endpoints: []string{grpcEndpoint},
	}
	discoveryInterval := time.Second * 5
	peerServer, err := privateapi.NewServer(n.ctx, myInfo, serverTLSConfig, disco, grpcPort, dnsProvider, dnsSuffix, clientTLSConfig, discoveryInterval)
	peerServer.PingInterval = time.Second
	peerServer.HealthyTimeout = time.Second * 5
	peerServer.DiscoveryPollInterval = time.Second * 5
	if err != nil {
		klog.Fatalf("error building server: %v", err)
	}

	scheme := "https"
	if n.InsecureMode {
		scheme = "http"
	}

	var clientUrls []string
	clientPort := 4001
	clientUrls = append(clientUrls, fmt.Sprintf("%s://%s:%d", scheme, address, clientPort))

	var quarantinedClientUrls []string
	quarantinedClientPort := 4002
	quarantinedClientUrls = append(quarantinedClientUrls, fmt.Sprintf("%s://%s:%d", scheme, address, quarantinedClientPort))

	var peerUrls []string
	peerPort := 2380
	peerUrls = append(peerUrls, fmt.Sprintf("%s://%s:%d", scheme, address, peerPort))

	me := &apis_etcd.EtcdNode{
		Name:                  string(uniqueID),
		ClientUrls:            clientUrls,
		QuarantinedClientUrls: quarantinedClientUrls,
		PeerUrls:              peerUrls,
	}

	backupStore, err := backup.NewStore(n.TestHarness.BackupStorePath)
	if err != nil {
		t.Fatalf("error initializing backup store: %v", err)
	}
	backupInterval := 15 * time.Minute

	commandStore, err := commands.NewStore(n.TestHarness.BackupStorePath)
	if err != nil {
		t.Fatalf("error initializing commands store: %v", err)
	}

	leaderLock, err := locking.NewFSContentLock(n.TestHarness.LockPath)
	if err != nil {
		t.Fatalf("error initializing lock: %v", err)
	}

	etcdClientsCA := n.TestHarness.etcdClientsCA
	etcdPeersCA := n.TestHarness.etcdPeersCA

	if n.InsecureMode {
		etcdClientsCA = nil
		etcdPeersCA = nil
	}

	var peerClientIPs []net.IP
	etcdServer, err := etcd.NewEtcdServer(n.NodeDir, n.TestHarness.ClusterName, n.Address, n.ListenMetricsURLs, me, peerServer, dnsProvider, etcdClientsCA, etcdPeersCA, peerClientIPs)
	if err != nil {
		t.Fatalf("error building EtcdServer: %v", err)
	}
	n.etcdServer = etcdServer
	go etcdServer.Run(n.ctx)

	// No automatic refreshes
	controlRefreshInterval := 10 * 365 * 24 * time.Hour

	c, err := controller.NewEtcdController(leaderLock, backupStore, backupInterval, commandStore, controlRefreshInterval, n.TestHarness.ClusterName, dnsSuffix, peerServer, n.TestHarness.etcdClientsCA, n.InsecureMode)
	if err != nil {
		t.Fatalf("error building etcd controller: %v", err)
	}
	c.CycleInterval = testCycleInterval
	n.etcdController = c
	go c.Run(n.ctx)

	if err := peerServer.ListenAndServe(n.ctx, grpcEndpoint); err != nil {
		if n.ctx.Done() == nil {
			t.Fatalf("error creating private API server: %v", err)
		}
	}
}

func (n *TestHarnessNode) ListMembers(ctx context.Context) ([]*etcdclient.EtcdProcessMember, error) {
	client, err := n.NewClient()
	if err != nil {
		return nil, fmt.Errorf("error building etcd client: %v", err)
	}
	defer client.Close()

	return client.ListMembers(ctx)
}

func (n *TestHarnessNode) NewClient() (*etcdclient.EtcdClient, error) {
	client, err := etcdclient.NewClient([]string{n.ClientURL}, n.etcdClientTLSConfig)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (n *TestHarnessNode) Close() error {
	if n.etcdServer != nil {
		_, err := n.etcdServer.StopEtcdProcessForTest()
		if err != nil {
			return err
		}
	}
	if n.ctxCancel != nil {
		n.ctxCancel()
		n.ctxCancel = nil
	}
	return nil
}

// AssertVersion asserts that the client reports the server version specified
func (n *TestHarnessNode) AssertVersion(t *testing.T, version string) {
	ctx := context.TODO()

	client, err := n.NewClient()
	if err != nil {
		n.TestHarness.T.Fatalf("error building etcd client: %v", err)
	}
	defer client.Close()

	actual, err := client.ServerVersion(ctx)
	if err != nil {
		t.Fatalf("error getting version from node: %v", err)
	}
	if actual != version {
		t.Fatalf("version was not as expected.  expected=%q, actual=%q", version, actual)
	}
}

func (n *TestHarnessNode) WaitForHealthy(timeout time.Duration) {
	description := fmt.Sprintf("wait for node %s to be healthy", n.Address)
	n.TestHarness.WaitFor(timeout, description, func() error {
		client, err := n.NewClient()
		if err != nil {
			return fmt.Errorf("error building etcd client: %v", err)
		}
		defer client.Close()

		_, err = client.ListMembers(context.Background())
		return err
	})
}

func (n *TestHarnessNode) WaitForHasLeader(timeout time.Duration) {
	description := fmt.Sprintf("wait for node %s to have a leader", n.Address)
	n.TestHarness.WaitFor(timeout, description, func() error {
		client, err := n.NewClient()
		if err != nil {
			return fmt.Errorf("error building etcd client: %v", err)
		}
		defer client.Close()

		leaderID, err := client.LeaderID(context.Background())
		if err != nil {
			return err
		}
		if leaderID == "" {
			return fmt.Errorf("node did not have leader")
		}
		return nil
	})
}

type MockDNSProvider struct {
}

var _ dns.Provider = &MockDNSProvider{}

func (p *MockDNSProvider) AddFallbacks(dnsFallbacks map[string][]net.IP) error {
	return nil
}

func (p *MockDNSProvider) UpdateHosts(addToHost map[string][]string) error {
	return nil
}
