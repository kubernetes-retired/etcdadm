package harness

import (
	"context"
	"fmt"
	"time"

	etcd_client "github.com/coreos/etcd/client"
	"github.com/golang/glog"
	"kope.io/etcd-manager/pkg/etcdclient"
)

func (n *TestHarnessNode) Get(ctx context.Context, key string) (string, error) {
	keysAPI, err := n.KeysAPI()
	if err != nil {
		return "", err
	}

	// TODO: Quorum?  Read from all nodes?
	opts := &etcd_client.GetOptions{
		Quorum: true,
	}
	response, err := keysAPI.Get(ctx, key, opts)
	if err != nil {
		return "", fmt.Errorf("error reading from member %s: %v", n.ClientURL, err)
	}
	if response.Node == nil {
		return "", nil
	}
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

func WaitForListMembers(client etcdclient.Client, timeout time.Duration) {
	endAt := time.Now().Add(timeout)
	for {
		members, err := client.ListMembers(context.Background())
		if err == nil {
			return
		}
		glog.Infof("Got members from %s: (%v, %v)", client, members, err)
		if time.Now().After(endAt) {
			break
		}
		time.Sleep(time.Second)
	}
}
