package harness

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/glog"
	"kope.io/etcd-manager/pkg/etcdclient"
)

func (n *TestHarnessNode) GetQuorum(ctx context.Context, key string) (string, error) {
	return n.get(ctx, key, true)
}

func (n *TestHarnessNode) GetLocal(ctx context.Context, key string) (string, error) {
	return n.get(ctx, key, false)
}

func (n *TestHarnessNode) get(ctx context.Context, key string, quorum bool) (string, error) {
	client, err := n.NewClient()
	if err != nil {
		return "", err
	}

	response, err := client.Get(ctx, key, quorum)
	if err != nil {
		return "", fmt.Errorf("error reading from member %s: %v", n.ClientURL, err)
	}
	glog.Infof("read from %q: %q", key, response)
	return string(response), nil
}

func (n *TestHarnessNode) Put(ctx context.Context, key string, value string) error {
	client, err := n.NewClient()
	if err != nil {
		return err
	}
	defer client.Close()

	err = client.Put(ctx, key, []byte(value))
	if err != nil {
		return fmt.Errorf("error writing to  %s: %v", n.ClientURL, err)
	}

	glog.Infof("etcd set %q = %q", key, value)

	return nil
}

func (n *TestHarnessNode) waitForClient(deadline time.Time) etcdclient.EtcdClient {
	t := n.TestHarness.T

	for {
		client, err := n.NewClient()
		if client != nil && err == nil {
			return client
		}

		if err != nil {
			glog.Warningf("error building client: %v", err)
		}

		glog.Infof("test waiting for client: (%v)", err)

		if time.Now().After(deadline) {
			t.Fatalf("wait-for-client did not succeed within timeout")
		}
		time.Sleep(time.Second)
	}
}

func (n *TestHarnessNode) WaitForListMembers(timeout time.Duration) {
	t := n.TestHarness.T

	endAt := time.Now().Add(timeout)

	client := n.waitForClient(endAt)

	for {
		members, err := client.ListMembers(context.Background())
		if err == nil {
			glog.Infof("Got members from %s: (%v)", client, members)
			return
		}
		glog.Infof("test waiting for members from %s: (%v)", client, err)

		if time.Now().After(endAt) {
			t.Fatalf("list-members did not succeed within %v", timeout)
			return
		}
		time.Sleep(time.Second)
	}
}

func (h *TestHarness) WaitFor(timeout time.Duration, f func() error) {
	t := h.T

	deadline := time.Now().Add(timeout)
	for {
		err := f()
		if err == nil {
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("time out waiting for condition: %v", err)
		} else {
			t.Logf("waiting for condition: %v", err)
		}

		time.Sleep(time.Second)
	}
}

func (n *TestHarnessNode) WaitForQuorumRead(ctx context.Context, timeout time.Duration) {
	client, err := n.NewClient()
	if err != nil {
		n.TestHarness.T.Fatalf("error building etcd client: %v", err)
		return
	}
	defer client.Close()
	endAt := time.Now().Add(timeout)
	for {
		_, err := n.GetQuorum(ctx, "/")
		if err == nil {
			glog.Infof("Got quorum-read on %q: (%v)", "/", client)
			return
		}
		glog.Infof("error from quorum-read on %q: %v", "/", err)
		if time.Now().After(endAt) {
			n.TestHarness.T.Fatalf("quorum-read did not succeed within %v", timeout)
			return
		}
		time.Sleep(time.Second)
	}
}
