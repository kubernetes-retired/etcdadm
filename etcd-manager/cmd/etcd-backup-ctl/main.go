/*
Copyright 2018 The Kubernetes Authors.

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

package main

import (
	"flag"
	"fmt"
	"os"

	"k8s.io/klog"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/backup"
)

func main() {
	klog.InitFlags(nil)

	backupStorePath := ""
	flag.StringVar(&backupStorePath, "backup-store", backupStorePath, "backup store location")

	flag.Parse()

	fmt.Printf("etcd-backup-ctl\n")

	if backupStorePath == "" {
		fmt.Fprintf(os.Stderr, "backup-store is required\n")
		os.Exit(1)
	}

	err := runBackupCtl(backupStorePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func runBackupCtl(backupStorePath string) error {
	backupStore, err := backup.NewStore(backupStorePath)
	if err != nil {
		return err
	}

	backups, err := backupStore.ListBackups()
	if err != nil {
		return err
	}

	for _, backup := range backups {
		fmt.Printf("%s\n", backup)
	}

	return nil
}
