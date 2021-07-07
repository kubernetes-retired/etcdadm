/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package harness

import (
	"context"
	"fmt"
	"time"

	"k8s.io/klog/v2"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/etcdclient"
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
	defer client.Close()

	response, err := client.Get(ctx, key, quorum)
	if err != nil {
		return "", fmt.Errorf("error reading from member %s: %v", n.ClientURL, err)
	}
	klog.Infof("read from %q: %q", key, response)
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

	klog.Infof("etcd set %q = %q", key, value)

	return nil
}

func (n *TestHarnessNode) waitForClient(deadline time.Time) *etcdclient.EtcdClient {
	t := n.TestHarness.T

	for {
		client, err := n.NewClient()
		if client != nil && err == nil {
			return client
		}

		if err != nil {
			klog.Warningf("error building client: %v", err)
		}

		klog.Infof("test waiting for client: (%v)", err)

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
			klog.Infof("Got members from %s: (%v)", client, members)
			return
		}
		klog.Infof("test waiting for members from %s: (%v)", client, err)

		if time.Now().After(endAt) {
			t.Fatalf("list-members did not succeed within %v", timeout)
			return
		}
		time.Sleep(time.Second)
	}
}

func (h *TestHarness) WaitFor(timeout time.Duration, description string, f func() error) {
	t := h.T

	deadline := time.Now().Add(timeout)
	for {
		err := f()
		if err == nil {
			return
		}

		// We also log to klog, so that it appears in the test output.
		if time.Now().After(deadline) {
			klog.Errorf("time out waiting for condition %q: %v", description, err)
			t.Fatalf("time out waiting for condition %q: %v", description, err)
		} else {
			klog.Infof("waiting for condition %q: %v", description, err)
			t.Logf("waiting for condition %q: %v", description, err)
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
			klog.Infof("Got quorum-read on %q: (%v)", "/", client)
			return
		}
		klog.Infof("error from quorum-read on %q: %v", "/", err)
		if time.Now().After(endAt) {
			n.TestHarness.T.Fatalf("quorum-read did not succeed within %v", timeout)
			return
		}
		time.Sleep(time.Second)
	}
}
