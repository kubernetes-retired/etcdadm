package integration

import (
	"context"
	"flag"
	"testing"
	"time"

	"kope.io/etcd-manager/pkg/etcdclient"
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

	h := NewTestHarness(t, ctx)
	h.MemberCount = 1
	defer h.Close()

	n1 := h.NewNode("127.0.0.1")
	go n1.Run()

	client := etcdclient.NewClient("http://127.0.0.1:4001")
	waitForListMembers(client, 20*time.Second)
	members, err := client.ListMembers(ctx)
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

	h := NewTestHarness(t, ctx)
	h.MemberCount = 3
	defer h.Close()

	n1 := h.NewNode("127.0.0.1")
	go n1.Run()
	n2 := h.NewNode("127.0.0.2")
	go n2.Run()
	n3 := h.NewNode("127.0.0.3")
	go n3.Run()

	client := etcdclient.NewClient("http://127.0.0.1:4001")
	waitForListMembers(client, 20*time.Second)
	members, err := client.ListMembers(ctx)
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

	h := NewTestHarness(t, ctx)
	h.MemberCount = 3
	defer h.Close()

	n1 := h.NewNode("127.0.0.1")
	go n1.Run()
	n2 := h.NewNode("127.0.0.2")
	go n2.Run()

	client1 := etcdclient.NewClient("http://127.0.0.1:4001")
	waitForListMembers(client1, 20*time.Second)
	members1, err := client1.ListMembers(ctx)
	if err != nil {
		t.Errorf("error doing etcd ListMembers: %v", err)
	} else if len(members1) != 2 {
		t.Errorf("members was not as expected: %v", err)
	}

	client2 := etcdclient.NewClient("http://127.0.0.2:4001")
	members2, err := client2.ListMembers(ctx)
	if err != nil {
		t.Errorf("error doing etcd ListMembers: %v", err)
	} else if len(members2) != 2 {
		t.Errorf("members was not as expected: %v", err)
	}

	n3 := h.NewNode("127.0.0.3")
	go n3.Run()

	client3 := etcdclient.NewClient("http://127.0.0.3:4001")
	waitForListMembers(client3, 30*time.Second)
	members3, err := client3.ListMembers(ctx)
	if err != nil {
		t.Errorf("error doing etcd ListMembers: %v", err)
	} else if len(members3) != 3 {
		t.Errorf("members was not as expected: %v", err)
	}

	cancel()
	h.Close()
}
