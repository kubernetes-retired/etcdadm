package harness

import (
	"context"
	"fmt"
	"testing"
	"time"

	etcd_client "github.com/coreos/etcd/client"
	"github.com/golang/glog"
	"kope.io/etcd-manager/pkg/etcdclient"
)

func (n *TestHarnessNode) GetQuorum(ctx context.Context, key string) (string, error) {
	opts := &etcd_client.GetOptions{
		Quorum: true,
	}
	return n.get(ctx, key, opts)
}

func (n *TestHarnessNode) GetLocal(ctx context.Context, key string) (string, error) {
	opts := &etcd_client.GetOptions{
		Quorum: false,
	}
	return n.get(ctx, key, opts)
}

func (n *TestHarnessNode) get(ctx context.Context, key string, opts *etcd_client.GetOptions) (string, error) {
	keysAPI, err := n.KeysAPI()
	if err != nil {
		return "", err
	}

	response, err := keysAPI.Get(ctx, key, opts)
	if err != nil {
		return "", fmt.Errorf("error reading from member %s: %v", n.ClientURL, err)
	}
	if response.Node == nil {
		return "", nil
	}
	glog.Infof("read from %q: %q", key, response.Node.Value)
	return response.Node.Value, nil
}

func (n *TestHarnessNode) Set(ctx context.Context, key string, value string) error {
	keysAPI, err := n.KeysAPI()
	if err != nil {
		return err
	}

	opts := &etcd_client.SetOptions{}
	_, err = keysAPI.Set(ctx, key, value, opts)
	if err != nil {
		return fmt.Errorf("error writing to  %s: %v", n.ClientURL, err)
	}

	glog.Infof("etcd set %q = %q", key, value)

	return nil
}

func (n *TestHarnessNode) KeysAPI() (etcd_client.KeysAPI, error) {
	clientUrls := []string{
		n.ClientURL,
	}
	cfg := etcd_client.Config{
		Endpoints: clientUrls,
		Transport: etcd_client.DefaultTransport,
		// set timeout per request to fail fast when the target endpoint is unavailable
		HeaderTimeoutPerRequest: time.Second,
	}
	etcdClient, err := etcd_client.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("error building etcd client for %s: %v", n.ClientURL, err)
	}

	keysAPI := etcd_client.NewKeysAPI(etcdClient)
	return keysAPI, nil
}

func waitForListMembers(t *testing.T, client etcdclient.EtcdClient, timeout time.Duration) {
	endAt := time.Now().Add(timeout)
	for {
		members, err := client.ListMembers(context.Background())
		if err == nil {
			glog.Infof("Got members from %s: (%v)", client, members)
			return
		}
		glog.Infof("Got error reading members from %s: (%v)", client, err)
		if time.Now().After(endAt) {
			t.Fatalf("list-members did not succeed within %v", timeout)
			return
		}
		time.Sleep(time.Second)
	}
}

func (n *TestHarnessNode) WaitForQuorumRead(ctx context.Context, timeout time.Duration) {
	client, err := etcdclient.NewClient(n.EtcdVersion, []string{n.ClientURL})
	if err != nil {
		n.TestHarness.T.Fatalf("error building etcd client: %v", err)
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
