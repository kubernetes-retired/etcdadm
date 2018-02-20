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
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/golang/glog"

	"kope.io/etcd-manager/pkg/backup"
	"kope.io/etcd-manager/pkg/backupcontroller"
)

func main() {
	flag.Set("logtostderr", "true")

	clusterName := ""
	flag.StringVar(&clusterName, "cluster-name", clusterName, "name of cluster")
	backupStorePath := "/backups"
	flag.StringVar(&backupStorePath, "backup-store", backupStorePath, "backup store location")
	dataDir := "/data"
	flag.StringVar(&dataDir, "data-dir", dataDir, "directory for storing etcd data")
	clientURL := "http://127.0.0.1:4001"
	flag.StringVar(&clientURL, "client-url", clientURL, "URL on which to connect to etcd")

	flag.Parse()

	fmt.Printf("etcd-backup agent\n")

	if clusterName == "" {
		fmt.Fprintf(os.Stderr, "cluster-name is required\n")
		os.Exit(1)
	}

	if backupStorePath == "" {
		fmt.Fprintf(os.Stderr, "backup-store is required\n")
		os.Exit(1)
	}

	ctx := context.TODO()

	backupStore, err := backup.NewStore(backupStorePath)
	if err != nil {
		glog.Fatalf("error initializing backup store: %v", err)
	}
	clientURLs := []string{clientURL}
	c, err := backupcontroller.NewBackupController(backupStore, clusterName, clientURLs, dataDir)
	if err != nil {
		glog.Fatalf("error building backup controller: %v", err)
	}

	c.Run(ctx)

	os.Exit(0)
}
