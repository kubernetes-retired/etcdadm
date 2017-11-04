package integration

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang/glog"
	apis_etcd "kope.io/etcd-manager/pkg/apis/etcd"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/pkg/backup"
	"kope.io/etcd-manager/pkg/controller"
	"kope.io/etcd-manager/pkg/etcd"
	"kope.io/etcd-manager/pkg/etcdclient"
	"kope.io/etcd-manager/pkg/privateapi"
)


// testCycleInterval is the cycle interval to use for tests.
// A shorter value here has two advantages: tests are faster, and it is less likely to mask problems
const testCycleInterval = time.Second

type TestHarness struct {
	T *testing.T

	ClusterName       string
	BackupStorePath   string
	DiscoveryStoreDir string

	MemberCount int

	WorkDir string

	Nodes map[string]*TestHarnessNode

	Context context.Context
}

func NewTestHarness(t *testing.T, ctx context.Context) *TestHarness {
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Errorf("error building tempdir: %v", err)
	}

	glog.Infof("Starting new testharness in %s", tmpDir)

	clusterName := "testharnesscluster"
	h := &TestHarness{
		T:           t,
		ClusterName: clusterName,
		WorkDir:     path.Join(tmpDir, clusterName),
		MemberCount: 3,
		Nodes:       make(map[string]*TestHarnessNode),
		Context:     ctx,
	}

	h.BackupStorePath = "file://" + filepath.Join(h.WorkDir, "backupstore")
	h.DiscoveryStoreDir = filepath.Join(h.WorkDir, "discovery")

	return h
}

func (h *TestHarness) Close() {
	t := h.T

	for k, node := range h.Nodes {
		glog.Infof("Terminating node %q", k)
		if err := node.Close(); err != nil {
			t.Errorf("error closing node %q: %v", k, err)
		}
	}

	if h.WorkDir != "" {
		if err := os.RemoveAll(h.WorkDir); err != nil {
			t.Errorf("unable to remove workdir: %v", err)
		}
	}
}

type TestHarnessNode struct {
	TestHarness *TestHarness
	Address     string
	NodeDir     string

	ClientURL string
	etcdServer     *etcd.EtcdServer
	etcdController *controller.EtcdController
}

func (h *TestHarness) NewNode(address string) *TestHarnessNode {
	t := h.T

	if h.Nodes[address] != nil {
		t.Fatalf("node already in harness: %q", address)
	}

	nodeDir := filepath.Join(h.WorkDir, "nodes", address)
	if err := os.MkdirAll(nodeDir, 0755); err != nil {
		t.Fatalf("error creating directory %s: %v", nodeDir, err)
	}

	n := &TestHarnessNode{
		TestHarness: h,
		Address:     address,
		NodeDir:     nodeDir,
	}

	h.Nodes[address] = n
	n.ClientURL = "http://" + address + ":4001"

	return n
}

func (h *TestHarnessNode) Run() {
	t := h.TestHarness.T
	ctx := h.TestHarness.Context

	address := h.Address

	glog.Infof("Starting node %q", address)

	uniqueID, err := privateapi.PersistentPeerId(h.NodeDir)
	if err != nil {
		t.Fatalf("error getting persistent peer id: %v", err)
	}

	grpcPort := 8000
	discoMe := privateapi.DiscoveryNode{
		ID: uniqueID,
	}
	discoMe.Addresses = append(discoMe.Addresses, privateapi.DiscoveryAddress{
		IP: fmt.Sprintf("%s:%d", h.Address, grpcPort),
	})
	disco, err := privateapi.NewFilesystemDiscovery(h.TestHarness.DiscoveryStoreDir, discoMe)
	if err != nil {
		glog.Fatalf("error building discovery: %v", err)
	}

	grpcAddress := fmt.Sprintf("%s:%d", address, grpcPort)
	myInfo := privateapi.PeerInfo{
		Id:        string(uniqueID),
		Addresses: []string{address},
	}
	peerServer, err := privateapi.NewServer(myInfo, disco)
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

	var peerUrls []string
	peerPort := 2380
	peerUrls = append(peerUrls, fmt.Sprintf("http://%s:%d", address, peerPort))

	me := &apis_etcd.EtcdNode{
		Name:       string(uniqueID),
		ClientUrls: clientUrls,
		PeerUrls:   peerUrls,
	}
	//c.Me = me
	//c.Nodes = append(c.Nodes, me)
	//c.ClusterToken = "etcd-cluster-token-" + c.ClusterName

	backupStore, err := backup.NewStore(h.TestHarness.BackupStorePath)
	if err != nil {
		t.Fatalf("error initializing backup store: %v", err)
	}

	etcdServer := etcd.NewEtcdServer(h.NodeDir, h.TestHarness.ClusterName, me, peerServer)
	h.etcdServer = etcdServer
	go etcdServer.Run(ctx)

	initState := &protoetcd.ClusterSpec{
		MemberCount: int32(h.TestHarness.MemberCount),
	}

	c, err := controller.NewEtcdController(backupStore, h.TestHarness.ClusterName, peerServer, controller.StaticInitialClusterSpecProvider(initState))
	c.CycleInterval = testCycleInterval
	if err != nil {
		t.Fatalf("error building etcd controller: %v", err)
	}
	h.etcdController = c
	go c.Run(ctx)

	if err := peerServer.ListenAndServe(ctx, grpcAddress); err != nil {
		if ctx.Done() == nil {
			t.Fatalf("error creating private API server: %v", err)
		}
	}
}

func (h *TestHarnessNode) Close() error {
	if h.etcdServer != nil {
		_, err := h.etcdServer.StopEtcdProcess()
		if err != nil {
			return err
		}
	}
	return nil
}

func waitForListMembers(client etcdclient.Client, timeout time.Duration) {
	endAt := time.Now().Add(timeout)
	for {
		members, err := client.ListMembers(context.Background())
		if err == nil {
			return
		}
		glog.Infof("Got members from %s: (%v, %v)", client, members, err)
		if time.Now().After(endAt) {
			break
		}
		time.Sleep(time.Second)
	}
}
