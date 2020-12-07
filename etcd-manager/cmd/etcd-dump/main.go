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
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"k8s.io/klog/v2"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/backup"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/etcd"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/etcd/dump"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/etcdclient"
)

func main() {
	klog.InitFlags(nil)

	datadir := ""
	flag.StringVar(&datadir, "data-dir", datadir, "data dir location")
	out := ""
	flag.StringVar(&out, "out", out, "output file")

	flag.Parse()

	fmt.Printf("etcd-dump\n")

	if datadir == "" {
		fmt.Fprintf(os.Stderr, "data-dir is required\n")
		os.Exit(1)
	}

	err := runDump(datadir, out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func runDump(backupFile string, out string) error {
	backupFile = strings.TrimSuffix(backupFile, "/")
	lastSlash := strings.LastIndex(backupFile, "/")
	if lastSlash == -1 {
		return fmt.Errorf("expected slash in %q", backupFile)
	}
	backupStoreBase := backupFile[:lastSlash]
	backupName := backupFile[lastSlash+1:]

	backupStore, err := backup.NewStore(backupStoreBase)
	if err != nil {
		return err
	}

	clusterToken := "restore-etcd-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	tempDir := filepath.Join(os.TempDir(), clusterToken)
	if err := os.MkdirAll(tempDir, 0700); err != nil {
		return fmt.Errorf("error creating tempdir %q: %v", tempDir, err)
	}

	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			klog.Warningf("error cleaning up tempdir %q: %v", tempDir, err)
		}
	}()

	process, err := etcd.RunEtcdFromBackup(backupStore, backupName, tempDir)
	if err != nil {
		return err
	}

	defer func() {
		klog.Infof("stopping etcd that was reading backup")
		err := process.Stop()
		if err != nil {
			klog.Warningf("unable to stop etcd process that was started for dump: %v", err)
		}
	}()

	var nodeSink etcdclient.NodeSink
	if out == "" {
		nodeSink, err = dump.NewStreamDumpSink(os.Stdout)
		if err != nil {
			return fmt.Errorf("unable to create stream: %v", err)
		}
	} else {
		nodeSink, err = dump.NewTarDumpSink(out)
		if err != nil {
			return fmt.Errorf("unable to create file %q: %v", out, err)
		}
	}

	sourceClient, err := process.NewClient()
	if err != nil {
		return fmt.Errorf("error building etcd client: %v", err)
	}
	defer sourceClient.Close()

	for i := 0; i < 60; i++ {
		ctx := context.TODO()
		_, err := sourceClient.Get(ctx, "/", true)
		if err == nil {
			break
		}
		klog.Infof("Waiting for etcd to start (%v)", err)
		time.Sleep(time.Second)
	}

	ctx := context.TODO()
	if n, err := sourceClient.CopyTo(ctx, nodeSink); err != nil {
		return fmt.Errorf("error copying keys to sink: %v", err)
	} else {
		klog.Infof("read %d keys", n)
	}

	if err := nodeSink.Close(); err != nil {
		return err
	}

	return nil
}
