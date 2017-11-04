package integration

import (
	"context"
	"flag"
	"testing"
	"time"

	"github.com/golang/glog"
	"kope.io/etcd-manager/test/integration/harness"
)

func init() {
	flag.Set("logtostderr", "true")
	flag.Set("v", "2")
	flag.Parse()
}

func TestClusterWithOneMember(t *testing.T) {
	ctx := context.TODO()
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)

	defer cancel()

	h := harness.NewTestHarness(t, ctx)
	h.MemberCount = 1
	defer h.Close()

	n1 := h.NewNode("127.0.0.1")
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
}

func TestClusterWithThreeMembers(t *testing.T) {
	ctx := context.TODO()
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)

	defer cancel()

	h := harness.NewTestHarness(t, ctx)
	h.MemberCount = 3
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
	h.MemberCount = 3
	defer h.Close()

	n1 := h.NewNode("127.0.0.1")
	go n1.Run()
	n2 := h.NewNode("127.0.0.2")
	go n2.Run()

	n1.WaitForListMembers(20 * time.Second)
	members1, err := n1.ListMembers(ctx)
	if err != nil {
		t.Errorf("error doing etcd ListMembers: %v", err)
	} else if len(members1) != 2 {
		t.Errorf("members was not as expected: %v", err)
	} else {
		glog.Infof("got members from #1: %v", members1)
	}

	members2, err := n2.ListMembers(ctx)
	if err != nil {
		t.Errorf("error doing etcd ListMembers: %v", err)
	} else if len(members2) != 2 {
		t.Errorf("members was not as expected: %v", err)
	} else {
		glog.Infof("got members from #2: %v", members2)
	}

	n3 := h.NewNode("127.0.0.3")
	go n3.Run()

	n3.WaitForListMembers(20 * time.Second)
	members3, err := n3.ListMembers(ctx)
	if err != nil {
		t.Errorf("error doing etcd ListMembers: %v", err)
	} else if len(members3) != 3 {
		t.Errorf("members was not as expected: %v", err)
	} else {
		glog.Infof("got members from #3: %v", members3)
	}

	cancel()
	h.Close()
}
