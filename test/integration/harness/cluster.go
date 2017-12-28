package harness

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang/glog"
)

// testCycleInterval is the cycle interval to use for tests.
// A shorter value here has two advantages: tests are faster, and it is less likely to mask problems
const testCycleInterval = time.Second

type TestHarness struct {
	T *testing.T

	ClusterName       string
	LockPath          string
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

	glog.Infof("Starting new testharness for %q in %s", t.Name(), tmpDir)

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

	h.LockPath = filepath.Join(h.WorkDir, "lock")
	if err := os.MkdirAll(h.WorkDir, 0755); err != nil {
		t.Fatalf("error creating directory %s: %v", h.LockPath, err)
	}

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

	h.Nodes[address] = n
	n.ClientURL = "http://" + address + ":4001"

	return n
}

// SpecKey returns the etcd key that holds the cluster spec
func (h *TestHarness) SpecKey() string {
	return "/kope.io/etcd-manager/" + h.ClusterName + "/spec"
}
