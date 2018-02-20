package etcd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/golang/glog"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/pkg/backup"
	"kope.io/etcd-manager/pkg/etcdclient"
)

// DoBackup performs a backup of etcd v2 or v3
func DoBackup(backupStore backup.Store, info *protoetcd.BackupInfo, dataDir string, clientUrls []string) (*protoetcd.DoBackupResponse, error) {
	etcdVersion := info.EtcdVersion
	if etcdVersion == "" {
		return nil, fmt.Errorf("EtcdVersion not set")
	}

	if etcdclient.IsV2(etcdVersion) {
		return DoBackupV2(backupStore, info, dataDir)
	} else {
		return DoBackupV3(backupStore, info, clientUrls)
	}
}

// DoBackupV2 performs a backup of etcd v2, it needs etcdctl available
func DoBackupV2(backupStore backup.Store, info *protoetcd.BackupInfo, dataDir string) (*protoetcd.DoBackupResponse, error) {
	etcdVersion := info.EtcdVersion

	if dataDir == "" {
		return nil, fmt.Errorf("dataDir must be set for etcd version 2")
	}
	if etcdVersion == "" {
		return nil, fmt.Errorf("EtcdVersion not set")
	}

	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, fmt.Errorf("error creating etcd backup temp directory: %v", err)
	}

	defer func() {
		err := os.RemoveAll(tempDir)
		if err != nil {
			glog.Warningf("error deleting backup temp directory %q: %v", tempDir, err)
		}
	}()

	binDir, err := BindirForEtcdVersion(etcdVersion, "etcdctl")
	if err != nil {
		return nil, fmt.Errorf("etdctl not available for version %q", etcdVersion)
	}

	backupDir := filepath.Join(tempDir, "data")

	c := exec.Command(filepath.Join(binDir, "etcdctl"))

	c.Args = append(c.Args, "backup")
	c.Args = append(c.Args, "--data-dir", dataDir)
	c.Args = append(c.Args, "--backup-dir", backupDir)
	glog.Infof("executing command %s %s", c.Path, c.Args)

	env := make(map[string]string)
	for k, v := range env {
		c.Env = append(c.Env, k+"="+v)
	}

	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Start(); err != nil {
		return nil, fmt.Errorf("error running etcdctl backup: %v", err)
	}
	processState, err := c.Process.Wait()
	if err != nil {
		return nil, fmt.Errorf("etcdctl backup returned an error: %v", err)
	}

	if !processState.Success() {
		return nil, fmt.Errorf("etcdctl backup returned a non-zero exit code")
	}

	tgzFile := filepath.Join(tempDir, "backup.tgz")
	if err := createTgz(tgzFile, backupDir); err != nil {
		return nil, err
	}
	return uploadBackup(backupStore, info, tgzFile)
}

// DoBackupV3 performs a backup of etcd v3; using the etcd v3 API
func DoBackupV3(backupStore backup.Store, info *protoetcd.BackupInfo, clientUrls []string) (*protoetcd.DoBackupResponse, error) {
	etcdVersion := info.EtcdVersion

	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, fmt.Errorf("error creating etcd backup temp directory: %v", err)
	}

	defer func() {
		err := os.RemoveAll(tempDir)
		if err != nil {
			glog.Warningf("error deleting backup temp directory %q: %v", tempDir, err)
		}
	}()

	client, err := etcdclient.NewClient(etcdVersion, clientUrls)
	if err != nil {
		return nil, fmt.Errorf("error building etcd client to etcd: %v", err)
	}

	snapshotFile := filepath.Join(tempDir, "snapshot.db.gz")
	glog.Infof("performing snapshot save to %s", snapshotFile)
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
	name, err := backupStore.AddBackup(srcFile, fmt.Sprintf("%.6d", sequence), info)
	if err != nil {
		return nil, fmt.Errorf("error copying backup to storage: %v", err)
	}

	response := &protoetcd.DoBackupResponse{
		Name: name,
	}
	glog.Infof("backup complete: %v", response)
	return response, nil
}
