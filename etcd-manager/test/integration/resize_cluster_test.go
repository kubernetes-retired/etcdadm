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
	"testing"
	"time"

	"k8s.io/klog/v2"
	protoetcd "sigs.k8s.io/etcdadm/etcd-manager/pkg/apis/etcd"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/etcdversions"
	"sigs.k8s.io/etcdadm/etcd-manager/test/integration/harness"
)

func TestResizeCluster(t *testing.T) {
	for _, etcdVersion := range etcdversions.AllEtcdVersions {
		t.Run("etcdVersion="+etcdVersion, func(t *testing.T) {
			ctx := context.TODO()
			ctx, cancel := context.WithTimeout(ctx, time.Second*180)

			defer cancel()

			h := harness.NewTestHarness(t, ctx)
			h.SeedNewCluster(&protoetcd.ClusterSpec{MemberCount: 1, EtcdVersion: etcdVersion})
			defer h.Close()

			n1 := h.NewNode("127.0.0.1")
			n1.EtcdVersion = etcdVersion
			go n1.Run()

			testKey := "/testkey"

			{
				n1.WaitForListMembers(60 * time.Second)
				h.WaitForHealthy(n1)
				members1, err := n1.ListMembers(ctx)
				if err != nil {
					t.Errorf("error doing etcd ListMembers: %v", err)
				} else if len(members1) != 1 {
					t.Errorf("members was not as expected: %v", members1)
				} else {
					klog.Infof("got members from #1: %v", members1)
				}

				if err := n1.Put(ctx, testKey, "singlev2"); err != nil {
					t.Fatalf("unable to set test key: %v", err)
				}

				n1.AssertVersion(t, etcdversions.EtcdVersionForAdoption(etcdVersion))
			}

			n2 := h.NewNode("127.0.0.2")
			n2.EtcdVersion = etcdVersion
			go n2.Run()
			n3 := h.NewNode("127.0.0.3")
			n3.EtcdVersion = etcdVersion
			go n3.Run()

			klog.Infof("expanding to cluster size 3")
			{
				h.SetClusterSpec(&protoetcd.ClusterSpec{MemberCount: 3, EtcdVersion: etcdVersion})
				h.InvalidateControlStore(n1, n2, n3)

				n1.EtcdVersion = etcdVersion
				n2.EtcdVersion = etcdVersion
				n3.EtcdVersion = etcdVersion
			}

			// Check cluster is stable (on the v3 api)
			{
				n1.WaitForListMembers(1 * time.Second)
				n2.WaitForListMembers(20 * time.Second)
				n3.WaitForListMembers(20 * time.Second)
				h.WaitForHealthy(n1, n2, n3)
				members1, err := n1.ListMembers(ctx)
				if err != nil {
					t.Errorf("error doing etcd ListMembers: %v", err)
				} else if len(members1) != 3 {
					t.Errorf("members was not as expected: %v", members1)
				} else {
					klog.Infof("got members from #1: %v", members1)
				}
			}

			// Sanity check values
			{
				v, err := n1.GetQuorum(ctx, testKey)
				if err != nil {
					t.Fatalf("error reading test key after upgrade: %v", err)
				}
				if v != "singlev2" {
					t.Fatalf("unexpected test key value after upgrade: %q", v)
				}

				n1.AssertVersion(t, etcdversions.EtcdVersionForAdoption(etcdVersion))
				n2.AssertVersion(t, etcdversions.EtcdVersionForAdoption(etcdVersion))
				n3.AssertVersion(t, etcdversions.EtcdVersionForAdoption(etcdVersion))
			}

			cancel()
			h.Close()
		})
	}
}
