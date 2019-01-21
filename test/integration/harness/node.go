package harness

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang/glog"
	"k8s.io/kops/util/pkg/vfs"

	apis_etcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/pkg/backup"
	"kope.io/etcd-manager/pkg/commands"
	"kope.io/etcd-manager/pkg/controller"
	"kope.io/etcd-manager/pkg/dns"
	"kope.io/etcd-manager/pkg/etcd"
	"kope.io/etcd-manager/pkg/etcdclient"
	"kope.io/etcd-manager/pkg/locking"
	"kope.io/etcd-manager/pkg/pki"
	"kope.io/etcd-manager/pkg/privateapi"
	"kope.io/etcd-manager/pkg/privateapi/discovery"
	vfsdiscovery "kope.io/etcd-manager/pkg/privateapi/discovery/vfs"
	"kope.io/etcd-manager/pkg/tlsconfig"
)

type TestHarnessNode struct {
	TestHarness *TestHarness
	Address     string
	NodeDir     string
	EtcdVersion string

	ctx       context.Context
	ctxCancel context.CancelFunc

	ClientURL      string
	etcdServer     *etcd.EtcdServer
	etcdController *controller.EtcdController
}

func (n *TestHarnessNode) Run() {
	t := n.TestHarness.T

	n.ctx, n.ctxCancel = context.WithCancel(n.TestHarness.Context)

	address := n.Address

	glog.Infof("Starting node %q", address)

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
		glog.Fatalf("error parsing discovery path %q: %v", n.TestHarness.DiscoveryStoreDir, err)
	}
	disco, err := vfsdiscovery.NewVFSDiscovery(p, discoMe)
	if err != nil {
		glog.Fatalf("error building discovery: %v", err)
	}

	store := pki.NewFSStore(filepath.Join(n.NodeDir, "pki"))
	keypairs := &pki.Keypairs{Store: store}
	keypairs.SetCA(n.TestHarness.CA)

	serverTLSConfig, err := tlsconfig.GRPCServerConfig(keypairs, string(uniqueID))
	if err != nil {
		t.Fatalf("error building server TLS config: %v", err)
	}

	clientTLSConfig, err := tlsconfig.GRPCClientConfig(keypairs, string(uniqueID))
	if err != nil {
		t.Fatalf("error building client TLS config: %v", err)
	}

	dnsProvider := &MockDNSProvider{}
	dnsSuffix := "mock.local"

	myInfo := privateapi.PeerInfo{
		Id:        string(uniqueID),
		Endpoints: []string{grpcEndpoint},
	}
	peerServer, err := privateapi.NewServer(n.ctx, myInfo, serverTLSConfig, disco, grpcPort, dnsProvider, dnsSuffix, clientTLSConfig)
	peerServer.PingInterval = time.Second
	peerServer.HealthyTimeout = time.Second * 5
	peerServer.DiscoveryPollInterval = time.Second * 5
	if err != nil {
		glog.Fatalf("error building server: %v", err)
	}

	//c := &apis_etcd.EtcdNode{
	//	DesiredClusterSize: 3,
	//	ClusterName:        "etcd-main",
	//
	//	ClientPort: 4001,
	//	PeerPort:   2380,
	//}
	var clientUrls []string
	clientPort := 4001
	clientUrls = append(clientUrls, fmt.Sprintf("http://%s:%d", address, clientPort))

	var quarantinedClientUrls []string
	quarantinedClientPort := 4002
	quarantinedClientUrls = append(quarantinedClientUrls, fmt.Sprintf("http://%s:%d", address, quarantinedClientPort))

	var peerUrls []string
	peerPort := 2380
	peerUrls = append(peerUrls, fmt.Sprintf("http://%s:%d", address, peerPort))

	me := &apis_etcd.EtcdNode{
		Name:                  string(uniqueID),
		ClientUrls:            clientUrls,
		QuarantinedClientUrls: quarantinedClientUrls,
		PeerUrls:              peerUrls,
	}
	//c.Me = me
	//c.Nodes = append(c.Nodes, me)
	//c.ClusterToken = "etcd-cluster-token-" + c.ClusterName

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

	etcdServer, err := etcd.NewEtcdServer(n.NodeDir, n.TestHarness.ClusterName, n.Address, me, peerServer, dnsProvider)
	if err != nil {
		t.Fatalf("error building EtcdServer: %v", err)
	}
	n.etcdServer = etcdServer
	go etcdServer.Run(n.ctx)

	// No automatic refreshes
	controlRefreshInterval := 10 * 365 * 24 * time.Hour

	c, err := controller.NewEtcdController(leaderLock, backupStore, backupInterval, commandStore, controlRefreshInterval, n.TestHarness.ClusterName, dnsSuffix, peerServer)
	c.CycleInterval = testCycleInterval
	if err != nil {
		t.Fatalf("error building etcd controller: %v", err)
	}
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
		n.TestHarness.T.Fatalf("error building etcd client: %v", err)
	}
	if client == nil {
		return nil, fmt.Errorf("unable to build etcd client")
	}
	defer client.Close()
	return client.ListMembers(ctx)
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
	t := n.TestHarness.T

	endAt := time.Now().Add(timeout)
	for {
		client, err := n.NewClient()
		if err != nil {
			n.TestHarness.T.Fatalf("error building etcd client: %v", err)
		}

		_, err = client.ListMembers(context.Background())
		client.Close()
		if err == nil {
			return
		}

		if time.Now().After(endAt) {
			t.Fatalf("wait-for-healthy did not succeed within %v", timeout)
			return
		}
		time.Sleep(time.Second)
	}
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
