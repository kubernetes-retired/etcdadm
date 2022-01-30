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
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/blang/semver/v4"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/klog/v2"
	"sigs.k8s.io/etcdadm/apis"
	protoetcd "sigs.k8s.io/etcdadm/etcd-manager/pkg/apis/etcd"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/backup"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/etcdclient"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/pki"
	"sigs.k8s.io/etcdadm/service"
)

var baseDirs = []string{"/opt"}
var isTest = false

func init() {
	// For bazel
	// TODO: Use a flag?

	// used to fix glog parse error.
	_ = flag.CommandLine.Parse([]string{})

	if os.Getenv("TEST_SRCDIR") != "" && os.Getenv("TEST_WORKSPACE") != "" {
		d := filepath.Join(os.Getenv("TEST_SRCDIR"), os.Getenv("TEST_WORKSPACE"))
		klog.Infof("found bazel binary location: %s", d)
		baseDirs = append(baseDirs, d)

		isTest = true
	}
}

// etcdProcess wraps a running etcd process
type etcdProcess struct {
	BinDir string

	// dataDir is the directory holding the etcd data files.
	dataDir string

	// CurrentDir is the directory in which we launch the binary (cwd)
	CurrentDir string

	PKIPeersDir   string
	PKIClientsDir string

	etcdClientsCA *pki.CA
	// etcdClientTLSConfig is the tls.Config we can use to talk to the etcd process,
	// including a client certificate & CA configuration (if needed)
	etcdClientTLSConfig *tls.Config

	// etcdVersion is the version of etcd we are running
	etcdVersion string

	CreateNewCluster bool

	// ForceNewCluster is used during a restore, and passes the --force-new-cluster argument
	ForceNewCluster bool

	Cluster    *protoetcd.EtcdCluster
	MyNodeName string

	// Quarantined indicates if this process should be quarantined - we will use the QuarantinedClientUrls if so
	Quarantined bool

	// DisableTLS is set if we should _not_ enable TLS.
	// It's done this way to fail-secure
	DisableTLS bool

	cmd *exec.Cmd

	mutex     sync.Mutex
	exitError error
	exitState *os.ProcessState

	// ListenAddress is the address we bind to
	ListenAddress string

	// ListenMetricsURLs is the set of urls we should listen for metrics on
	ListenMetricsURLs []string

	// IgnoreListenMetricsURLs if this is set, we will not set Metrics URL even if ENV is set.
	IgnoreListenMetricsURLs bool
}

// DataDir implements the EtcdProcess interface
func (p *etcdProcess) DataDir() string {
	return p.dataDir
}

// EtcdVersion implements the EtcdProcess interface
func (p *etcdProcess) EtcdVersion() string {
	return p.etcdVersion
}

func (p *etcdProcess) ExitState() (error, *os.ProcessState) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	return p.exitError, p.exitState
}

func (p *etcdProcess) Stop() error {
	if p.cmd == nil {
		klog.Warningf("received Stop when process not running")
		return nil
	}
	if err := p.cmd.Process.Kill(); err != nil {
		p.mutex.Lock()
		if p.exitState != nil {
			klog.Infof("Exited etcd: %v", p.exitState)
			return nil
		}
		p.mutex.Unlock()
		return fmt.Errorf("failed to kill process: %v", err)
	}

	for {
		klog.Infof("Waiting for etcd to exit")
		p.mutex.Lock()
		if p.exitState != nil {
			exitState := p.exitState
			p.mutex.Unlock()
			klog.Infof("Exited etcd: %v", exitState)
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

	var binDirs []string
	for _, baseDir := range baseDirs {
		binDir := filepath.Join(baseDir, "etcd-"+etcdVersion+"-"+runtime.GOOS+"-"+runtime.GOARCH)
		binDirs = append(binDirs, binDir)
	}

	if isTest {
		for _, baseDir := range baseDirs {
			platform := "linux_amd64_stripped"

			binDir := filepath.Join(baseDir, "external", "etcd_"+strings.Replace(etcdVersion, ".", "_", -1)+"_source", platform)
			binDirs = append(binDirs, binDir)
			binDir = filepath.Join(baseDir, "external", "etcd_"+strings.Replace(etcdVersion, ".", "_", -1)+"_source", cmd, platform)
			binDirs = append(binDirs, binDir)
			binDir = filepath.Join(baseDir, "external", "etcd_"+strings.Replace(etcdVersion, ".", "_", -1)+"_source", cmd+"_")
			binDirs = append(binDirs, binDir)
			binDir = filepath.Join(baseDir, "external", "etcd_"+strings.Replace(etcdVersion, ".", "_", -1)+"_source", cmd, cmd+"_")
			binDirs = append(binDirs, binDir)
		}
	}

	for _, binDir := range binDirs {
		etcdBinary := filepath.Join(binDir, cmd)
		_, err := os.Stat(etcdBinary)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			} else {
				return "", fmt.Errorf("error checking for %s at %s: %v", cmd, etcdBinary, err)
			}
		}
		return binDir, nil
	}

	return "", fmt.Errorf("unknown etcd version %s: not found in %v", etcdVersion, binDirs)
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

// Config builds the EtcdAdmConfig, but does not start the process
func (p *etcdProcess) Config() (*apis.EtcdAdmConfig, error) {
	me := p.findMyNode()
	if me == nil {
		return nil, fmt.Errorf("unable to find self node %q in %v", p.MyNodeName, p.Cluster.Nodes)
	}

	readyClientURLs, err := parseURLs(me.ClientUrls)
	if err != nil {
		return nil, err
	}

	quarantinedClientURLs, err := parseURLs(me.QuarantinedClientUrls)
	if err != nil {
		return nil, err
	}

	peerURLs, err := parseURLs(me.PeerUrls)
	if err != nil {
		return nil, err
	}

	clientURLs := readyClientURLs
	if p.Quarantined {
		clientURLs = quarantinedClientURLs
	}

	config := &apis.EtcdAdmConfig{}
	// TODO: config.GOMAXPROCS = runtime.NumCPU()

	env := make(map[string]string)
	env["ETCD_DATA_DIR"] = p.DataDir()
	config.DataDir = p.DataDir()

	// etcd 3.4 deprecates '--logger=capnslog'
	//   [WARNING] Deprecated '--logger=capnslog' flag is set; use '--logger=zap' flag instead
	config.Logger = "zap"
	env["ETCD_LOGGER"] = config.Logger
	config.LogOutputs = "stdout"
	env["ETCD_LOG_OUTPUTS"] = config.LogOutputs

	// etcd 3.2 requires that we listen on an IP, not a DNS name
	config.ListenPeerURLs = changeHost(peerURLs, p.ListenAddress)
	env["ETCD_LISTEN_PEER_URLS"] = config.ListenPeerURLs.String()
	config.ListenClientURLs = changeHost(clientURLs, p.ListenAddress)
	env["ETCD_LISTEN_CLIENT_URLS"] = config.ListenClientURLs.String()
	config.AdvertiseClientURLs = clientURLs
	env["ETCD_ADVERTISE_CLIENT_URLS"] = clientURLs.String()
	config.InitialAdvertisePeerURLs = peerURLs
	env["ETCD_INITIAL_ADVERTISE_PEER_URLS"] = peerURLs.String()

	// This is only supported in 3.3 and later, but by using an env var it simply won't be picked up
	if len(p.ListenMetricsURLs) != 0 {
		listenMetricsURLs, err := parseURLs(p.ListenMetricsURLs)
		if err != nil {
			return nil, err
		}
		config.ListenMetricsURLs = listenMetricsURLs
		env["ETCD_LISTEN_METRICS_URLS"] = config.ListenMetricsURLs.String()
	}

	if p.CreateNewCluster {
		env["ETCD_INITIAL_CLUSTER_STATE"] = "new"
		config.InitialClusterState = "new"
	} else {
		env["ETCD_INITIAL_CLUSTER_STATE"] = "existing"
		config.InitialClusterState = "existing"
	}

	// Disable the etcd2 endpoint (needed for etcd <= v3.3)
	env["ETCD_ENABLE_V2"] = "false"
	config.EnableV2 = "false"

	env["ETCD_NAME"] = p.MyNodeName
	config.Name = p.MyNodeName
	if p.Cluster.ClusterToken != "" {
		env["ETCD_INITIAL_CLUSTER_TOKEN"] = p.Cluster.ClusterToken
		config.InitialClusterToken = p.Cluster.ClusterToken
	}

	var initialCluster []string
	for _, node := range p.Cluster.Nodes {
		initialCluster = append(initialCluster, node.Name+"="+strings.Join(node.PeerUrls, ","))
	}
	config.InitialCluster = strings.Join(initialCluster, ",")
	env["ETCD_INITIAL_CLUSTER"] = config.InitialCluster

	// Avoid quorum loss
	env["ETCD_STRICT_RECONFIG_CHECK"] = "true"
	// config.StrictReconfigCheck is always set

	if p.PKIPeersDir != "" {
		env["ETCD_PEER_CLIENT_CERT_AUTH"] = "true"
		// config.PeerClientCertAuth is always true
		config.PeerTrustedCAFile = filepath.Join(p.PKIPeersDir, "ca.crt")
		env["ETCD_PEER_TRUSTED_CA_FILE"] = config.PeerTrustedCAFile
		config.PeerCertFile = filepath.Join(p.PKIPeersDir, "me.crt")
		env["ETCD_PEER_CERT_FILE"] = config.PeerCertFile
		config.PeerKeyFile = filepath.Join(p.PKIPeersDir, "me.key")
		env["ETCD_PEER_KEY_FILE"] = config.PeerKeyFile
	} else {
		klog.Warningf("using insecure configuration for etcd peers")
	}

	if p.PKIClientsDir != "" {
		env["ETCD_CLIENT_CERT_AUTH"] = "true"
		// config.ClientCertAuth = true is always true
		config.TrustedCAFile = filepath.Join(p.PKIClientsDir, "ca.crt")
		env["ETCD_TRUSTED_CA_FILE"] = config.TrustedCAFile
		config.CertFile = filepath.Join(p.PKIClientsDir, "server.crt")
		env["ETCD_CERT_FILE"] = config.CertFile
		config.KeyFile = filepath.Join(p.PKIClientsDir, "server.key")
		env["ETCD_KEY_FILE"] = config.KeyFile
	} else {
		klog.Warningf("using insecure configuration for etcd clients")
	}

	// etcd 3.5 had some corruption issues and recommends this setting,
	// which is planned as the default in 3.6
	version, err := semver.ParseTolerant(p.EtcdVersion)
	if err != nil {
		klog.Warningf("error parsing version %q: %v", p.EtcdVersion, err)
	}

	if version.Major == 3 && version.Minor == 5 {
		env["ETCD_EXPERIMENTAL_INITIAL_CORRUPT_CHECK"] = "true"
	}

	// This should be the last step before setting the env vars for the
	// command so that any param can be overwritten.
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "ETCD_") {
			envPair := strings.SplitN(e, "=", 2)
			klog.Infof("Overwriting etcd setting %s with value %s", envPair[0], envPair[1])
			env[envPair[0]] = envPair[1]
		}
	}

	//IgnoreListenMetricsURLs
	if p.IgnoreListenMetricsURLs {
		delete(env, "ETCD_LISTEN_METRICS_URLS")
		config.ListenMetricsURLs = nil
	}

	{
		configEnv, err := service.BuildEnvironmentMap(config)
		if err != nil {
			return nil, fmt.Errorf("failed to build environment config: %w", err)
		}
		if !reflect.DeepEqual(configEnv, env) {
			klog.Warningf("environment did not match: %v vs %v", env, configEnv)
			for k := range env {
				if configEnv[k] != env[k] {
					klog.Warningf("diff %s=%s vs %s", k, configEnv[k], env[k])
				}
			}
			for k := range configEnv {
				if configEnv[k] != env[k] {
					klog.Warningf("diff %s=%s vs %s", k, configEnv[k], env[k])
				}
			}
			return nil, fmt.Errorf("environment did not match: %v vs %v", env, configEnv)
		}
	}

	config.EtcdExecutable = filepath.Join(p.BinDir, "etcd")
	config.Version = p.etcdVersion

	return config, nil
}

func (p *etcdProcess) Start() error {
	config, err := p.Config()
	if err != nil {
		return err
	}

	c := exec.Command(path.Join(p.BinDir, "etcd"))
	c.Dir = p.CurrentDir

	if p.ForceNewCluster {
		c.Args = append(c.Args, "--force-new-cluster")
	}
	klog.Infof("executing command %s %s", c.Path, c.Args)

	env, err := service.BuildEnvironmentMap(config)
	if err != nil {
		return fmt.Errorf("failed to build environment config: %w", err)
	}

	for k, v := range env {
		c.Env = append(c.Env, k+"="+v)
	}
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Start(); err != nil {
		return fmt.Errorf("error starting etcd: %w", err)
	}

	klog.Infof("started etcd with datadir %s; pid=%d", p.DataDir(), c.Process.Pid)
	p.cmd = c

	go func() {
		processState, err := p.cmd.Process.Wait()
		if err != nil {
			klog.Warningf("etcd exited with error: %v", err)
		}
		p.mutex.Lock()
		p.exitState = processState
		p.exitError = err
		p.mutex.Unlock()
		exitCode := -2
		if processState != nil {
			exitCode = processState.ExitCode()
		}
		klog.Infof("etcd process exited (datadir %s; pid=%d); exitCode=%d, exitErr=%v", p.DataDir(), p.cmd.Process.Pid, exitCode, err)
	}()

	return nil
}

func changeHost(urls apis.URLList, host string) apis.URLList {
	var remapped apis.URLList
	for _, u := range urls {
		newHost := host
		if u.Port() != "" {
			newHost = net.JoinHostPort(newHost, u.Port())
		}
		r := u
		r.Host = newHost
		remapped = append(remapped, r)
	}
	return remapped
}

func parseURLs(urls []string) (apis.URLList, error) {
	var out apis.URLList
	for _, s := range urls {
		u, err := url.Parse(s)
		if err != nil {
			return nil, fmt.Errorf("error parsing url %q: %w", s, err)
		}
		out = append(out, *u)
	}
	return out, nil
}

func BuildTLSClientConfig(keypairs *pki.Keypairs, cn string) (*tls.Config, error) {
	ca := keypairs.CA()
	caPool := ca.CertPool()

	keypair, err := keypairs.EnsureKeypair("client", certutil.Config{
		CommonName: cn,
		Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})
	if err != nil {
		return nil, err
	}

	c := &tls.Config{
		RootCAs: caPool,
	}
	c.Certificates = append(c.Certificates, tls.Certificate{
		Certificate: [][]byte{keypair.Certificate.Raw},
		PrivateKey:  keypair.PrivateKey,
		Leaf:        keypair.Certificate,
	})

	return c, nil
}

func (p *etcdProcess) NewClient() (*etcdclient.EtcdClient, error) {
	var me *protoetcd.EtcdNode
	for _, node := range p.Cluster.Nodes {
		if node.Name == p.MyNodeName {
			me = node
		}
	}
	if me == nil {
		return nil, fmt.Errorf("unable to find self node %q in %v", p.MyNodeName, p.Cluster.Nodes)
	}

	clientURLs := me.ClientUrls
	if p.Quarantined {
		clientURLs = me.QuarantinedClientUrls
	}

	return NewClient(clientURLs, p.CurrentDir, p.etcdClientTLSConfig)
}

func NewClient(clientURLs []string, currentDir string, etcdClientTLSConfig *tls.Config) (*etcdclient.EtcdClient, error) {
	// If there are any relative paths, make them absolute for the client.
	// This is a workaround for https://github.com/etcd-io/etcd/issues/12450
	for i, clientURL := range clientURLs {
		scheme := ""
		if strings.HasPrefix(clientURL, "unix://") {
			scheme = "unix"
		} else if strings.HasPrefix(clientURL, "unixs://") {
			scheme = "unixs"
		}
		if scheme == "" {
			// Not unix domain socket
			continue
		}

		if strings.HasPrefix(clientURL, scheme+":///") {
			// Already absolute
			continue
		}

		path := strings.TrimPrefix(clientURL, scheme+"://")

		if currentDir == "" {
			return nil, fmt.Errorf("clientURL was set to relative path %q, but process directory was not set", clientURL)
		}

		if !filepath.IsAbs(currentDir) {
			return nil, fmt.Errorf("process directory %q was not absolute", currentDir)
		}

		rewrote := scheme + "://" + filepath.Join(currentDir, path)
		klog.Infof("rewrote client url %q to %q", clientURLs[i], rewrote)
		clientURLs[i] = rewrote
	}

	return etcdclient.NewClient(clientURLs, etcdClientTLSConfig)
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

	return DoBackup(store, info, p.dataDir, clientUrls, p.etcdClientTLSConfig)
}

// RestoreV3Snapshot calls etcdctl snapshot restore
func (p *etcdProcess) RestoreV3Snapshot(snapshotFile string) error {
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
	c.Args = append(c.Args, "--data-dir", p.dataDir)
	klog.Infof("executing command %s %s", c.Path, c.Args)

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

	klog.Infof("snapshot restore complete")
	return nil
}
