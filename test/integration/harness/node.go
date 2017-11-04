package harness

import (
	"fmt"

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

type TestHarnessNode struct {
	TestHarness *TestHarness
	Address     string
	NodeDir     string

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
	discoMe := privateapi.DiscoveryNode{
		ID: uniqueID,
	}
	discoMe.Addresses = append(discoMe.Addresses, privateapi.DiscoveryAddress{
		IP: fmt.Sprintf("%s:%d", n.Address, grpcPort),
	})
	disco, err := privateapi.NewFilesystemDiscovery(n.TestHarness.DiscoveryStoreDir, discoMe)
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

	backupStore, err := backup.NewStore(n.TestHarness.BackupStorePath)
	if err != nil {
		t.Fatalf("error initializing backup store: %v", err)
	}

	etcdServer := etcd.NewEtcdServer(n.NodeDir, n.TestHarness.ClusterName, me, peerServer)
	n.etcdServer = etcdServer
	go etcdServer.Run(ctx)

	initState := &protoetcd.ClusterSpec{
		MemberCount: int32(n.TestHarness.MemberCount),
	}

	c, err := controller.NewEtcdController(backupStore, n.TestHarness.ClusterName, peerServer, controller.StaticInitialClusterSpecProvider(initState))
	c.CycleInterval = testCycleInterval
	if err != nil {
		t.Fatalf("error building etcd controller: %v", err)
	}
	n.etcdController = c
	go c.Run(ctx)

	if err := peerServer.ListenAndServe(ctx, grpcAddress); err != nil {
		if ctx.Done() == nil {
			t.Fatalf("error creating private API server: %v", err)
		}
	}
}

func (n *TestHarnessNode) WaitForListMembers(timeout time.Duration) {
	client := etcdclient.NewClient(n.ClientURL)
	WaitForListMembers(client, timeout)
}

func (n *TestHarnessNode) Close() error {
	if n.etcdServer != nil {
		_, err := n.etcdServer.StopEtcdProcess()
		if err != nil {
			return err
		}
	}
	return nil
}
