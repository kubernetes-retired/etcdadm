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
	for _, backupEtcdVersion := range etcdversions.LatestEtcdVersions {
		restoreEtcdVersion := etcdversions.EtcdVersionForRestore(backupEtcdVersion)
		t.Run("backupEtcdVersion="+backupEtcdVersion+"/restoreEtcdVersion="+restoreEtcdVersion, func(t *testing.T) {
			ctx := context.TODO()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			// Create a new cluster
			backupClusterSpec := &protoetcd.ClusterSpec{MemberCount: 1, EtcdVersion: backupEtcdVersion}
			h1 := harness.NewTestHarness(t, ctx)
			// Very short backup interval so we don't have to wait too long for a backup!
			h1.BackupInterval = 5 * time.Second
			h1.SeedNewCluster(backupClusterSpec)
			defer h1.Close()

			n1 := h1.NewNode("127.0.0.1")
			defer n1.Close()

			klog.Infof("Starting backup node %v with etcd version %q", n1.Address, backupClusterSpec.EtcdVersion)
			go n1.Run()

			n1.WaitForListMembers(20 * time.Second)

			key := "/testing/backuprestore_" + backupEtcdVersion
			value := "world-" + backupEtcdVersion

			klog.Infof("Writing key %q=%q to backup node", key, value)
			err := n1.Put(ctx, key, value)
			if err != nil {
				t.Fatalf("error writing key %q: %v", key, err)
			}

			klog.Infof("Reading key %q from backup node", key)
			{
				actual, err := n1.GetQuorum(ctx, key)
				if err != nil {
					t.Fatalf("error reading key %q: %v", key, err)
				}
				if actual != value {
					t.Fatalf("could not read back key %q: %q vs %q", key, actual, value)
				}
				klog.Infof("Found key %q with value %q in backup", key, value)
			}

			klog.Infof("Waiting for backup")
			h1.WaitForBackup(t, 5*time.Minute)

			// We should be able to shut down the node, delete the local etcd data, and it should do a restore
			klog.Infof("Stopping backup node %v", n1.Address)
			if err := n1.Close(); err != nil {
				t.Fatalf("failed to stop backup node: %v", err)
			}
			klog.Infof("Removing backup node directory %q", n1.NodeDir)
			if err := os.RemoveAll(n1.NodeDir); err != nil {
				t.Fatalf("failed to delete dir for backup node: %v", err)
			}

			// Create a new cluster, but reuse the backups
			restoreClusterSpec := &protoetcd.ClusterSpec{MemberCount: 1, EtcdVersion: restoreEtcdVersion}
			h2 := harness.NewTestHarness(t, ctx)
			h2.BackupStorePath = h1.BackupStorePath
			h2.SeedNewCluster(restoreClusterSpec)
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
			klog.Infof("Adding command to restore backup %q", backupName)
			h2.AddCommand(&protoetcd.Command{
				Timestamp: time.Now().UnixNano(),
				RestoreBackup: &protoetcd.RestoreBackupCommand{
					ClusterSpec: restoreClusterSpec,
					Backup:      backupName,
				},
			})

			n2 := h2.NewNode("127.0.0.2")
			defer n2.Close()

			klog.Infof("Starting restore node %v with etcd version %q", n2.Address, restoreClusterSpec.EtcdVersion)
			go n2.Run()

			n2.WaitForListMembers(20 * time.Second)

			klog.Infof("Reading key %q from restore node", key)
			{
				actual, err := n2.GetQuorum(ctx, key)
				if err != nil {
					t.Fatalf("error rereading key %q: %v", key, err)
				}
				if actual != value {
					t.Fatalf("could not reread key %q: %q vs %q", key, actual, value)
				}
				klog.Infof("Found key %q with value %q in backup", key, value)
			}

			klog.Infof("Stopping restore node %v", n2.Address)
			if err := n2.Close(); err != nil {
				t.Fatalf("failed to stop restore node: %v", err)
			}
		})
	}
}
