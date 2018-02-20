package etcd

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/pkg/backup"
	"kope.io/etcd-manager/pkg/etcdclient"
)

// etcdProcess wraps a running etcd process
type etcdProcess struct {
	BinDir  string
	DataDir string

	// EtcdVersion is the version of etcd we are running
	EtcdVersion string

	CreateNewCluster bool

	// ForceNewCluster is used during a restore, and passes the --force-new-cluster argument
	ForceNewCluster bool

	Cluster    *protoetcd.EtcdCluster
	MyNodeName string

	// Quarantined indicates if this process should be quarantined - we will use the QuarantinedClientUrls if so
	Quarantined bool

	cmd *exec.Cmd

	mutex     sync.Mutex
	exitError error
	exitState *os.ProcessState
}

func (p *etcdProcess) ExitState() (error, *os.ProcessState) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	return p.exitError, p.exitState
}

func (p *etcdProcess) Stop() error {
	if p.cmd == nil {
		glog.Warningf("received Stop when process not running")
		return nil
	}
	if err := p.cmd.Process.Kill(); err != nil {
		p.mutex.Lock()
		if p.exitState != nil {
			glog.Infof("Exited etcd: %v", p.exitState)
			return nil
		}
		p.mutex.Unlock()
		return fmt.Errorf("failed to kill process: %v", err)
	}

	for {
		glog.Infof("Waiting for etcd to exit")
		p.mutex.Lock()
		if p.exitState != nil {
			exitState := p.exitState
			p.mutex.Unlock()
			glog.Infof("Exited etcd: %v", exitState)
			return nil
		}
		p.mutex.Unlock()
		time.Sleep(100 * time.Millisecond)
	}
}

// BindirForEtcdVersion returns the directory in which the etcd binary is located, for the specified version
// It returns an error if the specified version cannot be found
func BindirForEtcdVersion(etcdVersion string, cmd string) (string, error) {
	if !strings.HasPrefix(etcdVersion, "v") {
		etcdVersion = "v" + etcdVersion
	}
	binDir := filepath.Join("/opt", "etcd-"+etcdVersion+"-"+runtime.GOOS+"-"+runtime.GOARCH)
	etcdBinary := filepath.Join(binDir, cmd)
	_, err := os.Stat(etcdBinary)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("unknown etcd version (%s not found at %s)", cmd, etcdBinary)
		} else {
			return "", fmt.Errorf("error checking for %s at %s: %v", cmd, etcdBinary, err)
		}
	}
	return binDir, nil
}

func (p *etcdProcess) findMyNode() *protoetcd.EtcdNode {
	var me *protoetcd.EtcdNode
	for _, node := range p.Cluster.Nodes {
		if node.Name == p.MyNodeName {
			me = node
		}
	}
	return me
}

func (p *etcdProcess) Start() error {
	c := exec.Command(path.Join(p.BinDir, "etcd"))
	if p.ForceNewCluster {
		c.Args = append(c.Args, "--force-new-cluster")
	}
	glog.Infof("executing command %s %s", c.Path, c.Args)

	me := p.findMyNode()
	if me == nil {
		return fmt.Errorf("unable to find self node %q in %v", p.MyNodeName, p.Cluster.Nodes)
	}

	clientUrls := me.ClientUrls
	if p.Quarantined {
		clientUrls = me.QuarantinedClientUrls
	}
	env := make(map[string]string)
	env["ETCD_DATA_DIR"] = p.DataDir
	env["ETCD_LISTEN_PEER_URLS"] = strings.Join(me.PeerUrls, ",")
	env["ETCD_LISTEN_CLIENT_URLS"] = strings.Join(clientUrls, ",")
	env["ETCD_ADVERTISE_CLIENT_URLS"] = strings.Join(clientUrls, ",")
	env["ETCD_INITIAL_ADVERTISE_PEER_URLS"] = strings.Join(me.PeerUrls, ",")

	if p.CreateNewCluster {
		env["ETCD_INITIAL_CLUSTER_STATE"] = "new"
	} else {
		env["ETCD_INITIAL_CLUSTER_STATE"] = "existing"
	}

	// For etcd3, we disable the etcd2 endpoint
	// The etcd2 endpoint runs a weird "second copy" of etcd
	if !strings.HasPrefix(p.EtcdVersion, "2.") {
		env["ETCD_ENABLE_V2"] = "false"
	}

	env["ETCD_NAME"] = p.MyNodeName
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
	p.cmd = c

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

	return nil
}

func (p *etcdProcess) NewClient() (etcdclient.EtcdClient, error) {
	var me *protoetcd.EtcdNode
	for _, node := range p.Cluster.Nodes {
		if node.Name == p.MyNodeName {
			me = node
		}
	}
	if me == nil {
		return nil, fmt.Errorf("unable to find self node %q in %v", p.MyNodeName, p.Cluster.Nodes)
	}

	clientUrls := me.ClientUrls
	if p.Quarantined {
		clientUrls = me.QuarantinedClientUrls
	}

	return etcdclient.NewClient(p.EtcdVersion, clientUrls)
}

// isV2 checks if this is etcd v2
func (p *etcdProcess) isV2() bool {
	if p.EtcdVersion == "" {
		glog.Fatalf("EtcdVersion not set")
	}
	return etcdclient.IsV2(p.EtcdVersion)
}

// DoBackup performs a backup/snapshot of the data
func (p *etcdProcess) DoBackup(store backup.Store, info *protoetcd.BackupInfo) (*protoetcd.DoBackupResponse, error) {
	me := p.findMyNode()
	if me == nil {
		return nil, fmt.Errorf("unable to find self node %q in %v", p.MyNodeName, p.Cluster.Nodes)
	}

	clientUrls := me.ClientUrls
	if p.Quarantined {
		clientUrls = me.QuarantinedClientUrls
	}

	return DoBackup(store, info, p.DataDir, clientUrls)
}

// RestoreV3Snapshot calls etcdctl snapshot restore
func (p *etcdProcess) RestoreV3Snapshot(snapshotFile string) error {
	if p.isV2() {
		return fmt.Errorf("unexpected version when calling RestoreV2Snapshot: %q", p.EtcdVersion)
	}

	me := p.findMyNode()
	if me == nil {
		return fmt.Errorf("unable to find self node %q in %v", p.MyNodeName, p.Cluster.Nodes)
	}

	var initialCluster []string
	for _, node := range p.Cluster.Nodes {
		initialCluster = append(initialCluster, node.Name+"="+strings.Join(node.PeerUrls, ","))
	}

	c := exec.Command(path.Join(p.BinDir, "etcdctl"))
	c.Args = append(c.Args, "snapshot", "restore", snapshotFile)
	c.Args = append(c.Args, "--name", me.Name)
	c.Args = append(c.Args, "--initial-cluster", strings.Join(initialCluster, ","))
	c.Args = append(c.Args, "--initial-cluster-token", p.Cluster.ClusterToken)
	c.Args = append(c.Args, "--initial-advertise-peer-urls", strings.Join(me.PeerUrls, ","))
	c.Args = append(c.Args, "--data-dir", p.DataDir)
	glog.Infof("executing command %s %s", c.Path, c.Args)

	env := make(map[string]string)
	env["ETCDCTL_API"] = "3"
	for k, v := range env {
		c.Env = append(c.Env, k+"="+v)
	}

	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Start(); err != nil {
		return fmt.Errorf("error running etcdctl snapshot restore: %v", err)
	}
	processState, err := c.Process.Wait()
	if err != nil {
		return fmt.Errorf("etcdctl snapshot restore returned an error: %v", err)
	}
	if !processState.Success() {
		return fmt.Errorf("etcdctl snapshot restore returned a non-zero exit code")
	}

	glog.Infof("snapshot restore complete")
	return nil
}
