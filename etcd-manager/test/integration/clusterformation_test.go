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

package integration

import (
	"context"
	"flag"
	"testing"
	"time"

	"k8s.io/klog"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/pkg/etcdversions"
	"kope.io/etcd-manager/test/integration/harness"
)

func init() {
	logflags := flag.NewFlagSet("testing", flag.ExitOnError)

	klog.InitFlags(logflags)

	logflags.Set("logtostderr", "true")
	logflags.Set("v", "2")
	logflags.Parse([]string{})
}

func TestClusterWithOneMember(t *testing.T) {
	for _, etcdVersion := range etcdversions.AllEtcdVersions {
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
		klog.Infof("got members from #1: %v", members1)
	}

	n2.WaitForListMembers(20 * time.Second)
	members2, err := n2.ListMembers(ctx)
	if err != nil {
		t.Fatalf("error doing etcd ListMembers: %v", err)
	} else if len(members2) != 2 {
		t.Fatalf("members was not as expected: %v", members2)
	} else {
		klog.Infof("got members from #2: %v", members2)
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
		klog.Infof("got members from #3: %v", members3)
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
		klog.Infof("got members from #1: %v", members1)
	}

	n2 := h.NewNode("127.0.0.2")
	go n2.Run()

	if _, err := n2.ListMembers(ctx); err == nil {
		t.Errorf("did not expect second cluster to form: %v", err)
	}

	cancel()
	h.Close()
}
