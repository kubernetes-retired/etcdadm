package etcd

import (
	"fmt"
	"github.com/golang/glog"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/pkg/backup"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"
)

type etcdProcess struct {
	BinDir  string
	DataDir string

	ClientURL string

	CreateNewCluster bool
	//Version          string

	//ClusterName string
	//NodeName string
	//AdvertiseHostname string
	//
	//PeerPort int
	//
	//ClientListenHost string
	//ClientPort int
	//
	//ClusterToken string

	Cluster *protoetcd.EtcdCluster

	cmd *exec.Cmd

	mutex     sync.Mutex
	exitError error
	exitState *os.ProcessState
}

func (p *etcdProcess) Stop() error {
	if p.cmd == nil {
		glog.Warningf("received Stop when process not running")
		return nil
	}
	if err := p.cmd.Process.Kill(); err != nil {
		p.mutex.Lock()
		if p.exitState != nil {
			return nil
		}
		p.mutex.Unlock()
		return fmt.Errorf("failed to kill process: ", err)
	}

	for {
		p.mutex.Lock()
		if p.exitState != nil {
			p.mutex.Unlock()
			return nil
		}
		p.mutex.Unlock()
		time.Sleep(100 * time.Millisecond)
	}
}

func (p *etcdProcess) Start() error {
	c := exec.Command(path.Join(p.BinDir, "etcd"))
	glog.Infof("executing command %s %s", c.Path, c.Args)

	//clientPort := p.Cluster.ClientPort
	//peerPort := p.Cluster.PeerPort
	//
	//clientListenHost := ""
	//if clientListenHost == "" {
	//	clientListenHost = "0.0.0.0"
	//}

	//advertiseHostname := p.Cluster.Me.InternalName
	//if advertiseHostname == "" {
	//	advertiseHostname = "0.0.0.0"
	//}

	env := make(map[string]string)
	env["ETCD_DATA_DIR"] = p.DataDir
	env["ETCD_LISTEN_PEER_URLS"] = strings.Join(p.Cluster.Me.PeerUrls, ",")
	env["ETCD_LISTEN_CLIENT_URLS"] = strings.Join(p.Cluster.Me.ClientUrls, ",")
	env["ETCD_ADVERTISE_CLIENT_URLS"] = strings.Join(p.Cluster.Me.ClientUrls, ",")
	env["ETCD_INITIAL_ADVERTISE_PEER_URLS"] = strings.Join(p.Cluster.Me.PeerUrls, ",")

	if p.CreateNewCluster {
		env["ETCD_INITIAL_CLUSTER_STATE"] = "new"
	} else {
		env["ETCD_INITIAL_CLUSTER_STATE"] = "existing"
	}

	env["ETCD_NAME"] = p.Cluster.Me.Name
	if p.Cluster.ClusterToken != "" {
		env["ETCD_INITIAL_CLUSTER_TOKEN"] = p.Cluster.ClusterToken
	}

	var initialCluster []string
	for _, node := range p.Cluster.Nodes {
		initialCluster = append(initialCluster, node.Name+"="+strings.Join(node.PeerUrls, ","))
	}
	env["ETCD_INITIAL_CLUSTER"] = strings.Join(initialCluster, ",")

	for k, v := range env {
		c.Env = append(c.Env, k+"="+v)
	}
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	err := c.Start()
	if err != nil {
		return fmt.Errorf("error starting etcd: %v", err)
	}
	go func() {
		processState, err := p.cmd.Process.Wait()
		if err != nil {
			glog.Warningf("etcd exited with error: %v", err)
		}
		p.mutex.Lock()
		p.exitState = processState
		p.exitError = err
		p.mutex.Unlock()
	}()

	p.cmd = c
	return nil
}

func (p *etcdProcess) DoBackup(store backup.Store) (*protoetcd.DoBackupResponse, error) {
	response := &protoetcd.DoBackupResponse{}

	timestamp := time.Now().UTC().Format(time.RFC3339)

	tempDir, err := store.CreateBackupTempDir(timestamp)
	if err != nil {
		return nil, fmt.Errorf("error creating etcd backup temp directory: %v", err)
	}

	defer func() {
		if tempDir != "" {
			err := os.RemoveAll(tempDir)
			if err != nil {
				glog.Warningf("error deleting backup temp directory %q: %v", tempDir, err)
			}
		}
	}()

	c := exec.Command(path.Join(p.BinDir, "etcdctl"))
	c.Args = append(c.Args, "backup")
	c.Args = append(c.Args, "--data-dir", p.DataDir)
	c.Args = append(c.Args, "--backup-dir", tempDir)
	glog.Infof("executing command %s %s", c.Path, c.Args)

	//clientPort := p.Cluster.ClientPort
	//peerPort := p.Cluster.PeerPort
	//
	//clientListenHost := ""
	//if clientListenHost == "" {
	//	clientListenHost = "0.0.0.0"
	//}

	//advertiseHostname := p.Cluster.Me.InternalName
	//if advertiseHostname == "" {
	//	advertiseHostname = "0.0.0.0"
	//}

	env := make(map[string]string)
	//env["ETCD_DATA_DIR"] = p.DataDir
	//env["ETCD_LISTEN_PEER_URLS"] = strings.Join(p.Cluster.Me.PeerUrls, ",")
	//env["ETCD_LISTEN_CLIENT_URLS"] = strings.Join(p.Cluster.Me.ClientUrls, ",")
	//env["ETCD_ADVERTISE_CLIENT_URLS"] = strings.Join(p.Cluster.Me.ClientUrls, ",")
	//env["ETCD_INITIAL_ADVERTISE_PEER_URLS"] = strings.Join(p.Cluster.Me.PeerUrls, ",")
	//
	//if p.CreateNewCluster {
	//	env["ETCD_INITIAL_CLUSTER_STATE"] = "new"
	//} else {
	//	env["ETCD_INITIAL_CLUSTER_STATE"] = "existing"
	//}
	//
	//env["ETCD_NAME"] = p.Cluster.Me.Name
	//if p.Cluster.ClusterToken != "" {
	//	env["ETCD_INITIAL_CLUSTER_TOKEN"] = p.Cluster.ClusterToken
	//}
	//
	//var initialCluster []string
	//for _, node := range p.Cluster.Nodes {
	//	initialCluster = append(initialCluster, node.Name+"="+strings.Join(node.PeerUrls, ","))
	//}
	//env["ETCD_INITIAL_CLUSTER"] = strings.Join(initialCluster, ",")

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

	response.Name = timestamp

	err = store.AddBackup(response.Name, tempDir)
	if err != nil {
		return nil, fmt.Errorf("error copying backup to storage: %v", err)
	}

	glog.Infof("backup complete: %v", response)
	return response, nil
}
