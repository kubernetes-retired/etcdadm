/*
Copyright 2017 The Kubernetes Authors.

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

	"github.com/golang/glog"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/pkg/backup"
)

func main() {
	flag.Set("logtostderr", "true")

	memberCount := 1
	flag.IntVar(&memberCount, "members", memberCount, "initial cluster size; cluster won't start until we have a quorum of this size")
	backupStorePath := "/backups"
	flag.StringVar(&backupStorePath, "backup-store", backupStorePath, "backup store location")
	etcdVersion := "3.2.12"
	flag.StringVar(&etcdVersion, "etcd-version", etcdVersion, "etcd version")

	flag.Parse()

	fmt.Printf("etcd-manager-ctl\n")

	if backupStorePath == "" {
		fmt.Fprintf(os.Stderr, "backup-store is required\n")
		os.Exit(1)
	}

	backupStore, err := backup.NewStore(backupStorePath)
	if err != nil {
		glog.Fatalf("error initializing backup store: %v", err)
	}

	spec := &protoetcd.ClusterSpec{
		MemberCount: int32(memberCount),
		EtcdVersion: etcdVersion,
	}
	cmd := &protoetcd.Command{
		CreateNewCluster: &protoetcd.CreateNewClusterCommand{ClusterSpec: spec},
	}
	if err := backupStore.AddCommand(cmd); err != nil {
		glog.Fatalf("error building etcd controller: %v", err)
	}

	os.Exit(0)
}
