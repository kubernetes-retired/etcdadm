package integration

import (
	"context"
	"testing"
	"time"

	"github.com/golang/glog"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/test/integration/harness"
)

func TestUpgradeDowngrade(t *testing.T) {
	ctx := context.TODO()
	ctx, cancel := context.WithTimeout(ctx, time.Second*240)

	defer cancel()

	h := harness.NewTestHarness(t, ctx)
	h.SeedNewCluster(&protoetcd.ClusterSpec{MemberCount: 3, EtcdVersion: "2.2.1"})
	defer h.Close()

	n1 := h.NewNode("127.0.0.1")
	go n1.Run()
	n2 := h.NewNode("127.0.0.2")
	go n2.Run()
	n3 := h.NewNode("127.0.0.3")
	go n3.Run()

	testKey := "/testkey"

	{
		n1.WaitForListMembers(60 * time.Second)
		h.WaitForHealthy(n1, n2, n3)
		members1, err := n1.ListMembers(ctx)
		if err != nil {
			t.Errorf("error doing etcd ListMembers: %v", err)
		} else if len(members1) != 3 {
			t.Errorf("members was not as expected: %v", members1)
		} else {
			glog.Infof("got members from #1: %v", members1)
		}

		if err := n1.Put(ctx, testKey, "worldv2"); err != nil {
			t.Fatalf("unable to set test key: %v", err)
		}

		n1.AssertVersion(t, "2.2.1")
		n2.AssertVersion(t, "2.2.1")
		n3.AssertVersion(t, "2.2.1")
	}

	// Upgrade to 3.2.24
	glog.Infof("upgrading to 3.2.24")
	{
		h.SetClusterSpec(&protoetcd.ClusterSpec{MemberCount: 3, EtcdVersion: "3.2.24"})
		h.InvalidateControlStore(n1, n2, n3)

		n1.EtcdVersion = "3.2.24"
		n2.EtcdVersion = "3.2.24"
		n3.EtcdVersion = "3.2.24"
	}

	// Check cluster is stable (on the v3 api)
	{
		n1.WaitForListMembers(60 * time.Second)
		h.WaitForHealthy(n1, n2, n3)
		members1, err := n1.ListMembers(ctx)
		if err != nil {
			t.Errorf("error doing etcd ListMembers: %v", err)
		} else if len(members1) != 3 {
			t.Errorf("members was not as expected: %v", members1)
		} else {
			glog.Infof("got members from #1: %v", members1)
		}
	}

	// Sanity check values
	{
		v, err := n1.GetQuorum(ctx, testKey)
		if err != nil {
			t.Fatalf("error reading test key after upgrade: %v", err)
		}
		if v != "worldv2" {
			t.Fatalf("unexpected test key value after upgrade: %q", v)
		}

		if err := n1.Put(ctx, testKey, "worldv3"); err != nil {
			t.Fatalf("unable to set test key: %v", err)
		}

		n1.AssertVersion(t, "3.2.24")
		n2.AssertVersion(t, "3.2.24")
		n3.AssertVersion(t, "3.2.24")
	}

	// Downgrade to 2.2.1
	glog.Infof("downgrading to 2.2.1")
	{
		h.SetClusterSpec(&protoetcd.ClusterSpec{MemberCount: 3, EtcdVersion: "2.2.1"})
		h.InvalidateControlStore(n1, n2, n3)

		n1.EtcdVersion = "2.2.1"
		n2.EtcdVersion = "2.2.1"
		n3.EtcdVersion = "2.2.1"
	}

	// Check cluster is stable (on the v2 api)
	{
		n1.WaitForListMembers(60 * time.Second)
		h.WaitForHealthy(n1, n2, n3)
		members1, err := n1.ListMembers(ctx)
		if err != nil {
			t.Errorf("error doing etcd ListMembers: %v", err)
		} else if len(members1) != 3 {
			t.Errorf("members was not as expected: %v", members1)
		} else {
			glog.Infof("got members from #1: %v", members1)
		}
	}

	// Sanity check values
	{
		v, err := n1.GetQuorum(ctx, testKey)
		if err != nil {
			t.Fatalf("error reading test key after downgrade: %v", err)
		}
		if v != "worldv3" {
			t.Fatalf("unexpected test key value after upgrade: %q", v)
		}

		n1.AssertVersion(t, "2.2.1")
		n2.AssertVersion(t, "2.2.1")
		n3.AssertVersion(t, "2.2.1")
	}

	cancel()
	h.Close()
}
