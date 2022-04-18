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
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"k8s.io/klog/v2"
	protoetcd "sigs.k8s.io/etcdadm/etcd-manager/pkg/apis/etcd"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/backup"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/etcdclient"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/etcdversions"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/pki"
)

// DoRestore restores a backup from the backup store
func (s *EtcdServer) DoRestore(ctx context.Context, request *protoetcd.DoRestoreRequest) (*protoetcd.DoRestoreResponse, error) {
	// TODO: Don't restore without a signal that it's OK
	s.mutex.Lock()
	defer s.mutex.Unlock()

	response := &protoetcd.DoRestoreResponse{}

	if err := s.validateHeader(request.Header); err != nil {
		return nil, err
	}

	if s.process == nil {
		return nil, fmt.Errorf("etcd not running")
	}

	if request.Storage == "" {
		return nil, fmt.Errorf("Storage is required")
	}
	if request.BackupName == "" {
		return nil, fmt.Errorf("BackupName is required")
	}

	backupStore, err := backup.NewStore(request.Storage)
	if err != nil {
		return nil, err
	}

	clusterToken := "restore-etcd-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	tempDir := filepath.Join(os.TempDir(), clusterToken)
	if err := os.MkdirAll(tempDir, 0700); err != nil {
		return nil, fmt.Errorf("error creating tempdir %q: %v", tempDir, err)
	}

	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			klog.Warningf("error cleaning up tempdir %q: %v", tempDir, err)
		}
	}()

	p, err := RunEtcdFromBackup(backupStore, request.BackupName, tempDir)
	if err != nil {
		return nil, err
	}

	defer func() {
		klog.Infof("stopping etcd that was reading backup")
		err := p.Stop()
		if err != nil {
			klog.Warningf("unable to stop etcd process that was started for restore: %v", err)
		}
	}()

	destClient, err := s.process.NewClient()
	if err != nil {
		return nil, fmt.Errorf("error building etcd client for target: %v", err)
	}
	defer destClient.Close()

	if err := copyEtcd(context.TODO(), p, destClient); err != nil {
		return nil, err
	}

	return response, nil
}

func RunEtcdFromBackup(backupStore backup.Store, backupName string, basedir string) (*etcdProcess, error) {
	dataDir := filepath.Join(basedir, "data")
	pkiDir := filepath.Join(basedir, "pki")
	clusterToken := filepath.Base(dataDir)

	backupInfo, err := backupStore.LoadInfo(backupName)
	if err != nil {
		return nil, err
	}

	if strings.HasPrefix(backupInfo.EtcdVersion, "2.") {
		return nil, fmt.Errorf("this version of etcd-manager does not support etcd v2")
	}

	// V3 requires that data dir not exist
	downloadFile := filepath.Join(basedir, "download", "snapshot.db.gz")

	klog.Infof("Downloading backup %q to %s", backupName, downloadFile)
	if err := backupStore.DownloadBackup(backupName, downloadFile); err != nil {
		return nil, fmt.Errorf("error restoring backup: %v", err)
	}

	etcdVersion := backupInfo.EtcdVersion
	// A few known-safe restore-from-backup options
	{
		restoreWith := etcdversions.EtcdVersionForRestore(etcdVersion)
		if restoreWith != "" && restoreWith != etcdVersion {
			klog.Warningf("restoring backup from etcd %q, will restore with %q", etcdVersion, restoreWith)
			etcdVersion = restoreWith
		}
	}

	binDir, err := BindirForEtcdVersion(etcdVersion, "etcd")
	if err != nil {
		return nil, err
	}

	myNodeName := "restore"
	myNode := &protoetcd.EtcdNode{
		Name: myNodeName,
	}

	// Using unix domain sockets is more secure and avoids port conflicts.
	// We have to construct the path to look like a host:port until https://github.com/etcd-io/etcd/pull/12469 lands.

	// We have to set the etcd working directory so that we can pass a relative path as the socket
	currentDir := basedir

	clientPath := "127.0.0.1:8002"
	peerPath := "127.0.0.1:8003"
	myNode.ClientUrls = []string{"unixs://" + clientPath}
	myNode.PeerUrls = []string{"unixs://" + peerPath}

	p := &etcdProcess{
		CreateNewCluster: true,
		ForceNewCluster:  true,
		BinDir:           binDir,
		etcdVersion:      etcdVersion,
		dataDir:          dataDir,
		Cluster: &protoetcd.EtcdCluster{
			ClusterToken: clusterToken,
			Nodes:        []*protoetcd.EtcdNode{myNode},
		},
		IgnoreListenMetricsURLs: true, // Do not Set ListenMetricsURLs for restore to avoid port conflicts
		MyNodeName:              myNodeName,
		ListenAddress:           "127.0.0.1",
		DisableTLS:              false,
		CurrentDir:              currentDir,
	}

	etcdClientsCA, err := pki.NewCA(pki.NewInMemoryStore())
	if err != nil {
		return nil, fmt.Errorf("error building CA: %v", err)
	}

	etcdPeersCA, err := pki.NewCA(pki.NewInMemoryStore())
	if err != nil {
		return nil, fmt.Errorf("error building CA: %v", err)
	}

	var peerClientIPs []net.IP // We restore using a single localhost server, so no additional cert SANs needed

	if err := p.createKeypairs(etcdPeersCA, etcdClientsCA, pkiDir, myNode, peerClientIPs); err != nil {
		return nil, err
	}

	klog.Infof("restoring snapshot")

	snapshotFile := filepath.Join(basedir, "download", "snapshot.db")
	archive := &gzFile{File: downloadFile}
	if err := archive.expand(snapshotFile); err != nil {
		return nil, fmt.Errorf("error expanding snapshot: %v", err)
	}

	if err := p.RestoreV3Snapshot(snapshotFile); err != nil {
		return nil, err
	}

	klog.Infof("starting etcd to read backup")
	if err := p.Start(); err != nil {
		return nil, fmt.Errorf("error starting etcd: %v", err)
	}

	return p, nil
}

func copyEtcd(ctx context.Context, source *etcdProcess, dest etcdclient.NodeSink) error {
	sourceClient, err := source.NewClient()
	if err != nil {
		return fmt.Errorf("error building etcd client: %v", err)
	}
	defer sourceClient.Close()

	for i := 0; i < 60; i++ {
		_, err := sourceClient.Get(ctx, "/", true, 2*time.Second)
		if err == nil {
			break
		}

		exitError, exitState := source.ExitState()
		if exitError != nil || exitState != nil {
			return fmt.Errorf("source etcd process exited (state=%v): %w", exitState, exitError)
		}
		klog.Infof("Waiting for etcd to start (%v)", err)
		time.Sleep(time.Second)
	}

	klog.Infof("copying etcd keys from backup-restore process to new cluster")
	if n, err := sourceClient.CopyTo(ctx, dest); err != nil {
		return fmt.Errorf("error copying keys: %v", err)
	} else {
		klog.Infof("restored %d keys", n)
	}

	return nil
}
