/*
Copyright 2021 The Kubernetes Authors.

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
	"os"
	"testing"
	"time"

	"k8s.io/klog/v2"
	protoetcd "sigs.k8s.io/etcdadm/etcd-manager/pkg/apis/etcd"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/backup"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/etcdversions"
	"sigs.k8s.io/etcdadm/etcd-manager/test/integration/harness"
)

func TestBackupRestore(t *testing.T) {
	for _, etcdVersion := range etcdversions.AllEtcdVersions {
		t.Run("etcdVersion="+etcdVersion, func(t *testing.T) {
			ctx := context.TODO()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			clusterSpec := &protoetcd.ClusterSpec{MemberCount: 1, EtcdVersion: etcdVersion}

			h1 := harness.NewTestHarness(t, ctx)
			// Very short backup interval so we don't have to wait too long for a backup!
			h1.BackupInterval = 5 * time.Second
			h1.SeedNewCluster(clusterSpec)
			defer h1.Close()

			n1 := h1.NewNode("127.0.0.1")
			go n1.Run()

			n1.WaitForListMembers(20 * time.Second)

			key := "/testing/backuprestore_" + etcdVersion
			value := "world-" + etcdVersion

			err := n1.Put(ctx, key, value)
			if err != nil {
				t.Fatalf("error writing key %q: %v", key, err)
			}

			{
				actual, err := n1.GetQuorum(ctx, key)
				if err != nil {
					t.Fatalf("error reading key %q: %v", key, err)
				}
				if actual != value {
					t.Fatalf("could not read back key %q: %q vs %q", key, actual, value)
				}
			}

			h1.WaitForBackup(t, 5*time.Minute)

			// We should be able to shut down the node, delete the local etcd data, and it should do a restore
			if err := n1.Close(); err != nil {
				t.Fatalf("failed to stop node 1: %v", err)
			}

			klog.Infof("removing directory %q", n1.NodeDir)
			if err := os.RemoveAll(n1.NodeDir); err != nil {
				t.Fatalf("failed to delete dir for node 1: %v", err)
			}

			// Create a new cluster, but reuse the backups
			h2 := harness.NewTestHarness(t, ctx)
			h2.BackupStorePath = h1.BackupStorePath
			h2.SeedNewCluster(clusterSpec)
			defer h2.Close()

			backupStore, err := backup.NewStore(h2.BackupStorePath)
			if err != nil {
				t.Fatalf("error initializing backup store: %v", err)
			}

			backups, err := backupStore.ListBackups()
			if err != nil {
				t.Fatalf("error listing backups: %v", err)
			}
			if len(backups) == 0 {
				t.Fatalf("no backups: %v", err)
			}

			// We also have to send a restore backup command for a full DR (no common data) scenario
			backupName := backups[len(backups)-1]
			klog.Infof("adding command to restore backup %q", backupName)
			h2.AddCommand(&protoetcd.Command{
				Timestamp: time.Now().UnixNano(),
				RestoreBackup: &protoetcd.RestoreBackupCommand{
					ClusterSpec: clusterSpec,
					Backup:      backupName,
				},
			})

			n2 := h2.NewNode("127.0.0.2")
			defer n2.Close()

			klog.Infof("starting node %v", n2.Address)
			go n2.Run()

			n2.WaitForListMembers(20 * time.Second)

			klog.Infof("checking for key %q", key)
			{
				actual, err := n2.GetQuorum(ctx, key)
				if err != nil {
					t.Fatalf("error rereading key %q: %v", key, err)
				}
				if actual != value {
					t.Fatalf("could not reread key %q: %q vs %q", key, actual, value)
				}
			}

			klog.Infof("stopping node 2")
			if err := n2.Close(); err != nil {
				t.Fatalf("failed to stop node 2: %v", err)
			}
		})
	}
}
