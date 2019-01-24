package integration

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang/glog"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/test/integration/harness"
)

func TestEnableTLS(t *testing.T) {
	for _, etcdVersion := range []string{"2.2.1", "3.2.24"} {
		for _, nodeCount := range []int{1, 3} {
			t.Run("etcdVersion="+etcdVersion+",nodeCount="+strconv.Itoa(nodeCount), func(t *testing.T) {
				ctx := context.TODO()
				ctx, cancel := context.WithTimeout(ctx, time.Second*180)

				defer cancel()

				h := harness.NewTestHarness(t, ctx)
				h.SeedNewCluster(&protoetcd.ClusterSpec{MemberCount: int32(nodeCount), EtcdVersion: etcdVersion})
				defer h.Close()

				var nodes []*harness.TestHarnessNode
				for i := 1; i <= nodeCount; i++ {
					n := h.NewNode("127.0.0." + strconv.Itoa(i))
					n.EtcdVersion = etcdVersion
					n.InsecureMode = true
					if err := n.Init(); err != nil {
						t.Fatalf("error initializing node: %v", err)
					}
					go n.Run()
					nodes = append(nodes, n)
				}

				testKey := "/test"

				{
					nodes[0].WaitForListMembers(60 * time.Second)
					for _, n := range nodes {
						h.WaitForHealthy(n)
					}
					members1, err := nodes[0].ListMembers(ctx)
					if err != nil {
						t.Errorf("error doing etcd ListMembers: %v", err)
					} else if len(members1) != nodeCount {
						t.Errorf("members was not as expected: %v", members1)
					} else {
						glog.Infof("got members from #1: %v", members1)
					}

					for _, n := range nodes {
						n.AssertVersion(t, etcdVersion)
					}
				}

				// Set up some values to check basic functionality / data preservation
				for i, n := range nodes {
					err := n.Put(ctx, testKey+strconv.Itoa(i), "value"+strconv.Itoa(i))
					if err != nil {
						t.Fatalf("error reading test key: %v", err)
					}
				}

				glog.Infof("marking node secure")
				{
					for _, n := range nodes {
						// Restart n1 in secure mode
						if err := n.Close(); err != nil {
							t.Fatalf("failed to stop node: %v", err)
						}
						n.InsecureMode = false
						if err := n.Init(); err != nil {
							t.Fatalf("error initializing node: %v", err)
						}
						time.Sleep(time.Second)
						go n.Run()
					}

					nodes[0].WaitForListMembers(60 * time.Second)
					h.WaitForHealthy(nodes[0])

					for i, n := range nodes {
						h.WaitFor(120*time.Second, func() error {
							members, err := n.ListMembers(ctx)
							if err != nil {
								return fmt.Errorf("error doing etcd ListMembers: %v", err)
							} else if len(members) != nodeCount {
								return fmt.Errorf("members was not as expected: %v", members)
							} else {
								glog.Infof("got members from #%d: %v", i, members)
							}

							for _, m := range members {
								for _, u := range m.ClientURLs {
									if strings.Contains(u, "http://") {
										return fmt.Errorf("member had http:// url: %v", m)
									}
								}
								for _, u := range m.PeerURLs {
									if strings.Contains(u, "http://") {
										return fmt.Errorf("member had http:// url: %v", m)
									}
								}
							}

							// Sanity check values
							v, err := n.GetQuorum(ctx, testKey+strconv.Itoa(i))
							if err != nil {
								return fmt.Errorf("error reading test key after upgrade: %v", err)
							}
							if v != "value"+strconv.Itoa(i) {
								// Reading the wrong value is _never_ ok
								t.Fatalf("unexpected test key value after TLS enable: %q", v)
							}

							return nil
						})
					}
				}

				// When we turn on peer TLS, we can do that live via Raft,
				// but we have to bounce the processes (?)
				// Sometimes we'll catch the process bouncing, so we pause briefly and check everyone is online before continuing
				{
					time.Sleep(5 * time.Second)
					h.WaitForHealthy(nodes...)
				}

				// Check still can write
				for i, n := range nodes {
					if err := n.Put(ctx, testKey+strconv.Itoa(i), "updated"); err != nil {
						t.Fatalf("unable to set test key: %v", err)
					}
				}

				glog.Infof("success")

				cancel()
				h.Close()
			})
		}
	}
}
