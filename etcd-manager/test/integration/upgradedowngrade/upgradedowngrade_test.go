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

package upgradedowngrade

import (
	"context"
	"testing"
	"time"

	"k8s.io/klog/v2"
	protoetcd "sigs.k8s.io/etcdadm/etcd-manager/pkg/apis/etcd"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/etcdversions"
	"sigs.k8s.io/etcdadm/etcd-manager/test/integration/harness"
)

func TestUpgradeDowngrade(t *testing.T) {
	for _, fromVersion := range etcdversions.LatestEtcdVersions {
		for _, toVersion := range etcdversions.LatestEtcdVersions {
			if fromVersion == toVersion {
				continue
			}

			t.Run("from="+fromVersion+",to="+toVersion, func(t *testing.T) {
				ctx := context.TODO()
				ctx, cancel := context.WithTimeout(ctx, time.Second*240)
				defer cancel()

				h := harness.NewTestHarness(t, ctx)
				h.SeedNewCluster(&protoetcd.ClusterSpec{MemberCount: 3, EtcdVersion: fromVersion})
				defer h.Close()

				n1 := h.NewNode("127.0.0.1")
				n2 := h.NewNode("127.0.0.2")
				n3 := h.NewNode("127.0.0.3")

				n1.EtcdVersion = fromVersion
				n3.EtcdVersion = fromVersion
				n2.EtcdVersion = fromVersion

				go n1.Run()
				go n2.Run()
				go n3.Run()

				testKey := "/testkey"

				{
					n1.WaitForListMembers(60 * time.Second)
					h.WaitForHealthy(n1, n2, n3)
					h.WaitForHasLeader(n1, n2, n3)
					members1, err := n1.ListMembers(ctx)
					if err != nil {
						t.Errorf("error doing etcd ListMembers (before upgrade): %v", err)
					} else if len(members1) != 3 {
						t.Errorf("members was not as expected (before upgrade): %v", members1)
					} else {
						klog.Infof("got members from #1: %v", members1)
					}

					if err := n1.Put(ctx, testKey, "worldv2"); err != nil {
						t.Fatalf("unable to set test key before upgrade: %v", err)
					}

					n1.AssertVersion(t, fromVersion)
					n2.AssertVersion(t, fromVersion)
					n3.AssertVersion(t, fromVersion)
				}

				// Upgrade to new version
				klog.Infof("upgrading to %s", toVersion)
				{
					h.SetClusterSpec(&protoetcd.ClusterSpec{MemberCount: 3, EtcdVersion: toVersion})
					h.InvalidateControlStore(n1, n2, n3)

					n1.EtcdVersion = toVersion
					n2.EtcdVersion = toVersion
					n3.EtcdVersion = toVersion
				}

				// Check cluster is stable (on the v3 api)
				{
					h.WaitForVersion(60*time.Second, toVersion, n1, n2, n3)
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
					if v != "worldv2" {
						t.Fatalf("unexpected test key value after upgrade: %q", v)
					}

					if err := n1.Put(ctx, testKey, "worldv3"); err != nil {
						t.Fatalf("unable to set test key after upgrade: %v", err)
					}

					n1.AssertVersion(t, toVersion)
					n2.AssertVersion(t, toVersion)
					n3.AssertVersion(t, toVersion)
				}

				// Downgrade back to original version
				klog.Infof("downgrading to " + fromVersion)
				{
					h.SetClusterSpec(&protoetcd.ClusterSpec{MemberCount: 3, EtcdVersion: fromVersion})
					h.InvalidateControlStore(n1, n2, n3)

					n1.EtcdVersion = fromVersion
					n2.EtcdVersion = fromVersion
					n3.EtcdVersion = fromVersion
				}

				// Check cluster is stable (on the v2 api)
				{
					h.WaitForVersion(60*time.Second, fromVersion, n1, n2, n3)
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
						t.Fatalf("error reading test key after downgrade: %v", err)
					}
					if v != "worldv3" {
						t.Fatalf("unexpected test key value after downgrade: %q", v)
					}

					n1.AssertVersion(t, fromVersion)
					n2.AssertVersion(t, fromVersion)
					n3.AssertVersion(t, fromVersion)
				}

				klog.Infof("success!")

				cancel()
				h.Close()
			})
		}
	}
}
