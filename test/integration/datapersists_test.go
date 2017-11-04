package integration

import (
	"context"
	"testing"
	"time"

	"kope.io/etcd-manager/pkg/etcdclient"
	"kope.io/etcd-manager/test/integration/harness"
)

func TestClusterDataPersists(t *testing.T) {
	ctx := context.TODO()
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)

	defer cancel()

	h := harness.NewTestHarness(t, ctx)
	h.MemberCount = 1
	defer h.Close()

	n1 := h.NewNode("127.0.0.1")
	go n1.Run()

	client := etcdclient.NewClient("http://127.0.0.1:4001")
	harness.WaitForListMembers(client, 20*time.Second)

	key := "/testing/hello"

	value := "world"

	err := n1.Set(ctx, key, value)
	if err != nil {
		t.Fatalf("error writing key %q: %v", key, err)
	}

	actual, err := n1.Get(ctx, key)
	if err != nil {
		t.Fatalf("error reading key %q: %v", key, err)
	}

	if actual != value {
		t.Fatalf("could not read back key %q: %q vs %q", key, actual, value)
	}

	cancel()
	h.Close()
}
