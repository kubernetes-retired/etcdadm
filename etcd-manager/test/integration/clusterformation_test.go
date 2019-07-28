package integration

import (
	"context"
	"flag"
	"testing"
	"time"

	"github.com/golang/glog"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/test/integration/harness"
)

func init() {
	flag.Set("logtostderr", "true")
	flag.Set("v", "2")
	flag.Parse()
}

var AllEtcdVersions = []string{"2.2.1", "3.1.12", "3.2.18", "3.2.24", "3.3.10", "3.3.13"}

func TestClusterWithOneMember(t *testing.T) {
	for _, etcdVersion := range AllEtcdVersions {
		t.Run("etcdVersion="+etcdVersion, func(t *testing.T) {
			ctx := context.TODO()
			ctx, cancel := context.WithTimeout(ctx, time.Second*30)

			defer cancel()

			h := harness.NewTestHarness(t, ctx)
			h.SeedNewCluster(&protoetcd.ClusterSpec{MemberCount: 1, EtcdVersion: etcdVersion})
			defer h.Close()

			n1 := h.NewNode("127.0.0.1")
			n1.EtcdVersion = etcdVersion
			go n1.Run()

			n1.WaitForListMembers(20 * time.Second)
			members, err := n1.ListMembers(ctx)
			if err != nil {
				t.Errorf("error doing etcd ListMembers: %v", err)
			}
			if len(members) != 1 {
				t.Errorf("members was not as expected: %v", members)
			}

			cancel()
			h.Close()
		})
	}

}

func TestClusterWithThreeMembers(t *testing.T) {
	ctx := context.TODO()
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)

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

	n1.WaitForListMembers(20 * time.Second)
	members, err := n1.ListMembers(ctx)
	if err != nil {
		t.Errorf("error doing etcd ListMembers: %v", err)
	}
	if len(members) != 3 {
		t.Errorf("members was not as expected: %v", members)
	}

	cancel()
	h.Close()
}

func TestClusterExpansion(t *testing.T) {
	ctx := context.TODO()
	ctx, cancel := context.WithTimeout(ctx, time.Second*60)

	defer cancel()

	h := harness.NewTestHarness(t, ctx)
	h.SeedNewCluster(&protoetcd.ClusterSpec{MemberCount: 3, EtcdVersion: "2.2.1"})
	defer h.Close()

	n1 := h.NewNode("127.0.0.1")
	go n1.Run()
	n2 := h.NewNode("127.0.0.2")
	go n2.Run()

	n1.WaitForListMembers(20 * time.Second)
	members1, err := n1.ListMembers(ctx)
	if err != nil {
		t.Fatalf("error doing etcd ListMembers: %v", err)
	} else if len(members1) != 2 {
		t.Fatalf("members was not as expected: %v", members1)
	} else {
		glog.Infof("got members from #1: %v", members1)
	}

	n2.WaitForListMembers(20 * time.Second)
	members2, err := n2.ListMembers(ctx)
	if err != nil {
		t.Fatalf("error doing etcd ListMembers: %v", err)
	} else if len(members2) != 2 {
		t.Fatalf("members was not as expected: %v", members2)
	} else {
		glog.Infof("got members from #2: %v", members2)
	}

	n3 := h.NewNode("127.0.0.3")
	go n3.Run()

	n3.WaitForListMembers(20 * time.Second)
	members3, err := n3.ListMembers(ctx)
	if err != nil {
		t.Fatalf("error doing etcd ListMembers: %v", err)
	} else if len(members3) != 3 {
		t.Fatalf("members was not as expected: %v", members3)
	} else {
		glog.Infof("got members from #3: %v", members3)
	}

	cancel()
	h.Close()
}

func TestWeOnlyFormASingleCluster(t *testing.T) {
	ctx := context.TODO()
	ctx, cancel := context.WithTimeout(ctx, time.Second*60)

	defer cancel()

	h := harness.NewTestHarness(t, ctx)
	h.SeedNewCluster(&protoetcd.ClusterSpec{MemberCount: 1, EtcdVersion: "2.2.1"})
	defer h.Close()

	n1 := h.NewNode("127.0.0.1")
	go n1.Run()

	n1.WaitForListMembers(20 * time.Second)
	members1, err := n1.ListMembers(ctx)
	if err != nil {
		t.Errorf("error doing etcd ListMembers: %v", err)
	} else if len(members1) != 1 {
		t.Errorf("members was not as expected: %v", members1)
	} else {
		glog.Infof("got members from #1: %v", members1)
	}

	n2 := h.NewNode("127.0.0.2")
	go n2.Run()

	if _, err := n2.ListMembers(ctx); err == nil {
		t.Errorf("did not expect second cluster to form: %v", err)
	}

	cancel()
	h.Close()
}
