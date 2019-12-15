package harness

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"k8s.io/klog"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/pkg/commands"
	"kope.io/etcd-manager/pkg/pki"
)

// testCycleInterval is the cycle interval to use for tests.
// A shorter value here has two advantages: tests are faster, and it is less likely to mask problems
const testCycleInterval = time.Second

type TestHarness struct {
	T *testing.T

	grpcCA        *pki.Keypair
	etcdClientsCA *pki.Keypair
	etcdPeersCA   *pki.Keypair

	ClusterName       string
	LockPath          string
	BackupStorePath   string
	DiscoveryStoreDir string

	WorkDir string

	Nodes map[string]*TestHarnessNode

	Context context.Context
}

func NewTestHarness(t *testing.T, ctx context.Context) *TestHarness {
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Errorf("error building tempdir: %v", err)
	}

	klog.Infof("Starting new testharness for %q in %s", t.Name(), tmpDir)

	clusterName := "testharnesscluster"
	h := &TestHarness{
		T:           t,
		ClusterName: clusterName,
		WorkDir:     path.Join(tmpDir, clusterName),
		Nodes:       make(map[string]*TestHarnessNode),
		Context:     ctx,
	}

	{
		store := pki.NewFSStore(filepath.Join(h.WorkDir, "pki/grpc"))
		keypairs := pki.Keypairs{Store: store}
		ca, err := keypairs.CA()
		if err != nil {
			t.Fatalf("error building CA: %v", err)
		}
		h.grpcCA = ca
	}

	{
		store := pki.NewFSStore(filepath.Join(h.WorkDir, "pki/clients"))
		keypairs := pki.Keypairs{Store: store}
		ca, err := keypairs.CA()
		if err != nil {
			t.Fatalf("error building CA: %v", err)
		}
		h.etcdClientsCA = ca
	}

	{
		store := pki.NewFSStore(filepath.Join(h.WorkDir, "pki/peers"))
		keypairs := pki.Keypairs{Store: store}
		ca, err := keypairs.CA()
		if err != nil {
			t.Fatalf("error building CA: %v", err)
		}
		h.etcdPeersCA = ca
	}

	// To test with S3:
	// TEST_VFS_BASE_DIR=s3://bucket/etcd-manager/testing/ go test ./test/... -args --v=2 -logtostderr

	baseDir := os.Getenv("TEST_VFS_BASE_DIR")
	if baseDir == "" {
		h.BackupStorePath = "file://" + filepath.Join(h.WorkDir, "backupstore")
		h.DiscoveryStoreDir = filepath.Join(h.WorkDir, "discovery")
	} else {
		tmp := time.Now().Format(time.RFC3339)
		h.BackupStorePath = baseDir + "/" + tmp + "/backupstore"
		h.DiscoveryStoreDir = baseDir + "/" + tmp + "/discovery"
	}

	h.LockPath = filepath.Join(h.WorkDir, "lock")
	if err := os.MkdirAll(h.WorkDir, 0755); err != nil {
		t.Fatalf("error creating directory %s: %v", h.LockPath, err)
	}

	return h
}

func (h *TestHarness) Close() {
	t := h.T

	for k, node := range h.Nodes {
		klog.Infof("Terminating node %q", k)
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
		EtcdVersion: "2.2.1",
	}
	if err := n.Init(); err != nil {
		t.Fatalf("error initializing node: %v", err)
	}

	h.Nodes[address] = n

	return n
}

func (h *TestHarness) WaitForHealthy(nodes ...*TestHarnessNode) {
	for _, node := range nodes {
		node.WaitForHealthy(10 * time.Second)
	}
}

func (h *TestHarness) WaitForVersion(timeout time.Duration, expectedVersion string, nodes ...*TestHarnessNode) {
	for _, n := range nodes {
		h.WaitFor(timeout, func() error {
			client, err := n.NewClient()
			if err != nil {
				return fmt.Errorf("error building etcd client: %v", err)
			}

			version, err := client.ServerVersion(h.Context)
			client.Close()
			if err != nil {
				return fmt.Errorf("error getting etcd version: %v", err)
			}

			if version == expectedVersion {
				klog.Infof("node %q is on target version %q", n.Address, expectedVersion)
				return nil
			}

			return fmt.Errorf("version %q was not target version %q", version, expectedVersion)
		})
	}
}

func (h *TestHarness) SeedNewCluster(spec *protoetcd.ClusterSpec) {
	t := h.T
	controlStore, err := commands.NewStore(h.BackupStorePath)
	if err != nil {
		t.Fatalf("error initializing control store: %v", err)
	}
	if err := controlStore.SetExpectedClusterSpec(spec); err != nil {
		t.Fatalf("error setting cluster spec: %v", err)
	}
}

func (h *TestHarness) SetClusterSpec(spec *protoetcd.ClusterSpec) {
	t := h.T
	controlStore, err := commands.NewStore(h.BackupStorePath)
	if err != nil {
		t.Fatalf("error initializing control store: %v", err)
	}
	if err := controlStore.SetExpectedClusterSpec(spec); err != nil {
		t.Fatalf("error setting cluster spec: %v", err)
	}
}

func (h *TestHarness) InvalidateControlStore(nodes ...*TestHarnessNode) {
	t := h.T

	for _, node := range nodes {
		for {
			// Wait for goroutines to start
			if node.etcdController != nil {
				break
			}
			time.Sleep(time.Second)
		}

		err := node.etcdController.InvalidateControlStore()
		if err != nil {
			t.Fatalf("error invaliding control store: %v", err)
		}
	}
}
