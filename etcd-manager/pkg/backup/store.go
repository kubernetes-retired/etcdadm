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

package backup

import (
	"fmt"
	"k8s.io/kops/util/pkg/vfs"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/apis/etcd"
)

const MetaFilename = "_etcd_backup.meta"
const DataFilename = "etcd.backup.gz"

type Store interface {
	Spec() string

	// AddBackup adds a backup to the store, returning the name of the backup
	AddBackup(backupFile string, sequence string, info *etcd.BackupInfo) (string, error)

	// ListBackups returns all the available backups, in chronological order
	ListBackups() ([]string, error)

	// RemoveBackup deletes a backup (as returned by ListBackups)
	RemoveBackup(backup string) error

	// LoadInfo loads the backup information that should have been saved alongside a backup
	LoadInfo(backup string) (*etcd.BackupInfo, error)

	// DownloadBackup downloads the backup to the specific file
	DownloadBackup(name string, destFile string) error
}

func NewStore(storage string) (Store, error) {
	//u, err := url.Parse(storage)
	//if err != nil {
	//	return nil, fmt.Errorf("error parsing storage url %q", storage)
	//}
	//
	//switch u.Scheme {
	//case "file":
	//	return NewFilesystemStore(u)
	//
	//default:
	//	return nil, fmt.Errorf("unknown storage scheme %q", storage)
	//}

	fmt.Printf("storage = %s\n", storage)
	p, err := vfs.Context.BuildVfsPath(storage)
	fmt.Printf("path built = %v\npath() = %s", p, p.Path())
	if err != nil {
		return nil, err
	}
	return NewVFSStore(p)
}
