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

package etcd

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/klog/v2"
	protoetcd "sigs.k8s.io/etcdadm/etcd-manager/pkg/apis/etcd"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/backup"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/etcdclient"
)

// DoBackup performs a backup of etcd v2 or v3
func DoBackup(backupStore backup.Store, info *protoetcd.BackupInfo, dataDir string, clientUrls []string, tlsConfig *tls.Config) (*protoetcd.DoBackupResponse, error) {
	etcdVersion := info.EtcdVersion
	if etcdVersion == "" {
		return nil, fmt.Errorf("EtcdVersion not set")
	}

	return DoBackupV3(backupStore, info, clientUrls, tlsConfig)
}

// DoBackupV3 performs a backup of etcd v3; using the etcd v3 API
func DoBackupV3(backupStore backup.Store, info *protoetcd.BackupInfo, clientUrls []string, tlsConfig *tls.Config) (*protoetcd.DoBackupResponse, error) {
	tempDir, err := os.MkdirTemp("", "")
	if err != nil {
		return nil, fmt.Errorf("error creating etcd backup temp directory: %v", err)
	}

	defer func() {
		err := os.RemoveAll(tempDir)
		if err != nil {
			klog.Warningf("error deleting backup temp directory %q: %v", tempDir, err)
		}
	}()

	client, err := etcdclient.NewClient(clientUrls, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("error building etcd client to etcd: %v", err)
	}
	defer etcdclient.LoggedClose(client)

	snapshotFile := filepath.Join(tempDir, "snapshot.db.gz")
	klog.Infof("performing snapshot save to %s", snapshotFile)
	if err := client.SnapshotSave(context.TODO(), snapshotFile); err != nil {
		return nil, fmt.Errorf("error performing snapshot save: %v", err)
	}

	return uploadBackup(backupStore, info, snapshotFile)
}

// sequence is used to provide a tie-breaker for backups that happen in less than one second, primarily.
var sequence = 0

// uploadBackup uploads a backup directory to a backup.Store
func uploadBackup(backupStore backup.Store, info *protoetcd.BackupInfo, srcFile string) (*protoetcd.DoBackupResponse, error) {
	sequence++
	if sequence > 999999 {
		sequence = 0
	}
	backupName, err := backupStore.AddBackup(srcFile, fmt.Sprintf("%.6d", sequence), info)
	if err != nil {
		return nil, fmt.Errorf("error copying backup to storage: %v", err)
	}

	response := &protoetcd.DoBackupResponse{
		Name: backupName,
	}
	klog.Infof("backup complete: %v", response)
	return response, nil
}
