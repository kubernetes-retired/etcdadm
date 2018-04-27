package harness

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/golang/glog"
	"k8s.io/kops/util/pkg/vfs"

	apis_etcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/pkg/backup"
	"kope.io/etcd-manager/pkg/controller"
	"kope.io/etcd-manager/pkg/etcd"
	"kope.io/etcd-manager/pkg/etcdclient"
	"kope.io/etcd-manager/pkg/locking"
	"kope.io/etcd-manager/pkg/privateapi"
	"kope.io/etcd-manager/pkg/privateapi/discovery"
	vfsdiscovery "kope.io/etcd-manager/pkg/privateapi/discovery/vfs"
)

type TestHarnessNode struct {
	TestHarness *TestHarness
	Address     string
	NodeDir     string
	EtcdVersion string

	ClientURL      string
	etcdServer     *etcd.EtcdServer
	etcdController *controller.EtcdController
}

func (n *TestHarnessNode) Run() {
	t := n.TestHarness.T
	ctx := n.TestHarness.Context

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
		Endpoint: grpcEndpoint,
	})
	p, err := vfs.Context.BuildVfsPath(n.TestHarness.DiscoveryStoreDir)
	if err != nil {
		glog.Fatalf("error parsing discovery path %q: %v", n.TestHarness.DiscoveryStoreDir, err)
	}
	disco, err := vfsdiscovery.NewVFSDiscovery(p, discoMe)
	if err != nil {
		glog.Fatalf("error building discovery: %v", err)
	}

	myInfo := privateapi.PeerInfo{
		Id:        string(uniqueID),
		Endpoints: []string{grpcEndpoint},
	}
	peerServer, err := privateapi.NewServer(ctx, myInfo, disco)
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

	leaderLock, err := locking.NewFSContentLock(n.TestHarness.LockPath)
	if err != nil {
		t.Fatalf("error initializing lock: %v", err)
	}

	etcdServer := etcd.NewEtcdServer(n.NodeDir, n.TestHarness.ClusterName, me, peerServer)
	n.etcdServer = etcdServer
	go etcdServer.Run(ctx)

	c, err := controller.NewEtcdController(leaderLock, backupStore, n.TestHarness.ClusterName, peerServer)
	c.CycleInterval = testCycleInterval
	if err != nil {
		t.Fatalf("error building etcd controller: %v", err)
	}
	n.etcdController = c
	go c.Run(ctx)

	if err := peerServer.ListenAndServe(ctx, grpcEndpoint); err != nil {
		if ctx.Done() == nil {
			t.Fatalf("error creating private API server: %v", err)
		}
	}
}

func (n *TestHarnessNode) WaitForListMembers(timeout time.Duration) {
	client, err := etcdclient.NewClient(n.EtcdVersion, []string{n.ClientURL})
	if err != nil {
		n.TestHarness.T.Fatalf("error building etcd client: %v", err)
	}
	defer client.Close()
	waitForListMembers(n.TestHarness.T, client, timeout)
}

func (n *TestHarnessNode) ListMembers(ctx context.Context) ([]*etcdclient.EtcdProcessMember, error) {
	client, err := etcdclient.NewClient(n.EtcdVersion, []string{n.ClientURL})
	if err != nil {
		n.TestHarness.T.Fatalf("error building etcd client: %v", err)
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
	return nil
}

// CheckVersion asserts that the client reports the server version specified
func (n *TestHarnessNode) AssertVersion(t *testing.T, version string) {
	ctx := context.TODO()

	client, err := etcdclient.NewClient(n.EtcdVersion, []string{n.ClientURL})
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
		client, err := etcdclient.NewClient(n.EtcdVersion, []string{n.ClientURL})
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
