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

package apis

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	uuid "github.com/satori/go.uuid"
	"sigs.k8s.io/etcdadm/constants"

	netutil "k8s.io/apimachinery/pkg/util/net"
)

// EtcdAdmConfig holds etcdadm configuration
type EtcdAdmConfig struct {
	Version         string
	ReleaseURL      string
	CertificatesDir string

	DownloadConnectTimeout time.Duration

	PeerCertFile      string
	PeerKeyFile       string
	PeerTrustedCAFile string

	CertFile      string
	KeyFile       string
	TrustedCAFile string

	EtcdctlCertFile string
	EtcdctlKeyFile  string

	DataDir    string
	InstallDir string
	CacheDir   string

	BindAddr string

	UnitFile            string
	EnvironmentFile     string
	EtcdExecutable      string
	EtcdctlExecutable   string
	EtcdctlEnvFile      string
	EtcdctlShellWrapper string
	EtcdDiskPriorities  []string

	InitialAdvertisePeerURLs URLList
	ListenPeerURLs           URLList
	AdvertiseClientURLs      URLList
	ListenClientURLs         URLList

	LoopbackClientURL url.URL

	// ServerCertSANs sets extra Subject Alternative Names for the etcd server signing cert.
	ServerCertSANs []string
	// PeerCertSANs sets extra Subject Alternative Names for the etcd peer signing cert.
	PeerCertSANs []string

	Name                string
	InitialCluster      string
	InitialClusterToken string
	InitialClusterState string

	// GOMAXPROCS sets the max num of etcd processes will use
	GOMAXPROCS int

	Snapshot      string
	SkipHashCheck bool

	// Retry sets enable or disable backoff retry when join etcd member to cluster.
	// Default true, it mean that enable backoff retry.
	Retry bool
}

// EndpointStatus TODO: add description
type EndpointStatus struct {
	EtcdMember
	// TODO(dlipovetsky) See https://github.com/coreos/etcd/blob/52ae578922ee3d63b23c797b61beba041573ce1a/etcdctl/ctlv3/command/ep_command.go#L117
	// Health string `json:"health,omitempty"`
}

// EtcdMember TODO: add description
type EtcdMember struct {
	// ID is the member ID for this member.
	ID uint64 `json:"ID,omitempty"`
	// name is the human-readable name of the member. If the member is not started, the name will be an empty string.
	Name string `json:"name,omitempty"`
	// peerURLs is the list of URLs the member exposes to the cluster for communication.
	PeerURLs []string `json:"peerURLs,omitempty"`
	// clientURLs is the list of URLs the member exposes to clients for communication. If the member is not started, clientURLs will be empty.
	ClientURLs []string `json:"clientURLs,omitempty"`
}

// URLList TODO: add description
type URLList []url.URL

func (l URLList) String() string {
	stringURLs := l.StringSlice()
	return strings.Join(stringURLs, ",")
}

// StringSlice TODO: add description
func (l URLList) StringSlice() []string {
	stringURLs := make([]string, len(l))
	for i, url := range l {
		stringURLs[i] = url.String()
	}
	return stringURLs
}

// SetInfoDynamicDefaults checks and sets configuration values used by the info verb
func SetInfoDynamicDefaults(cfg *EtcdAdmConfig) error {
	return setDynamicDefaults(cfg)
}

// SetInitDynamicDefaults checks and sets configuration values used by the init verb
func SetInitDynamicDefaults(cfg *EtcdAdmConfig) error {
	cfg.InitialClusterState = "new"
	if err := setDynamicDefaults(cfg); err != nil {
		return err
	}
	cfg.InitialClusterToken = uuid.NewV4().String()[0:8]
	cfg.InitialCluster = fmt.Sprintf("%s=%s", cfg.Name, cfg.InitialAdvertisePeerURLs)
	return nil
}

// SetJoinDynamicDefaults checks and sets configuration values used by the join verb
func SetJoinDynamicDefaults(cfg *EtcdAdmConfig) error {
	cfg.InitialClusterState = "existing"
	return setDynamicDefaults(cfg)
}

// SetResetDynamicDefaults checks and sets configuration values used by the reset verb
func SetResetDynamicDefaults(cfg *EtcdAdmConfig) error {
	return setDynamicDefaults(cfg)
}

// SetDownloadDynamicDefaults checks and sets configuration values used by the download verb
func SetDownloadDynamicDefaults(cfg *EtcdAdmConfig) error {
	return setDynamicDefaults(cfg)
}

// SetDefaults sets configuration values defined at build time
func SetDefaults(cfg *EtcdAdmConfig) {
	cfg.DataDir = constants.DefaultDataDir
	cfg.UnitFile = constants.UnitFile
	cfg.EnvironmentFile = constants.EnvironmentFile
	cfg.EtcdctlEnvFile = constants.EtcdctlEnvFile
}

func setDynamicDefaults(cfg *EtcdAdmConfig) error {
	if len(cfg.Name) == 0 {
		name, err := os.Hostname()
		if err != nil {
			return fmt.Errorf("unable to use hostname as default name: %s", err)
		}
		cfg.Name = name
	}

	cfg.CacheDir = filepath.Join(constants.DefaultCacheBaseDir, "etcd", fmt.Sprintf("v%s", cfg.Version))
	cfg.EtcdExecutable = filepath.Join(cfg.InstallDir, "etcd")
	cfg.EtcdctlExecutable = filepath.Join(cfg.InstallDir, "etcdctl")
	cfg.EtcdctlShellWrapper = filepath.Join(cfg.InstallDir, "etcdctl.sh")

	cfg.PeerCertFile = filepath.Join(cfg.CertificatesDir, constants.EtcdPeerCertName)
	cfg.PeerKeyFile = filepath.Join(cfg.CertificatesDir, constants.EtcdPeerKeyName)
	cfg.PeerTrustedCAFile = filepath.Join(cfg.CertificatesDir, constants.EtcdCACertName)

	cfg.CertFile = filepath.Join(cfg.CertificatesDir, constants.EtcdServerCertName)
	cfg.KeyFile = filepath.Join(cfg.CertificatesDir, constants.EtcdServerKeyName)
	cfg.TrustedCAFile = filepath.Join(cfg.CertificatesDir, constants.EtcdCACertName)

	cfg.EtcdctlCertFile = filepath.Join(cfg.CertificatesDir, constants.EtcdctlClientCertName)
	cfg.EtcdctlKeyFile = filepath.Join(cfg.CertificatesDir, constants.EtcdctlClientKeyName)

	cfg.GOMAXPROCS = runtime.NumCPU()

	if err := DefaultPeerURLs(cfg); err != nil {
		return err
	}
	if err := DefaultClientURLs(cfg); err != nil {
		return err
	}
	DefaultPeerCertSANs(cfg)
	DefaultServerCertSANs(cfg)
	return nil
}

// DefaultServerCertSANs TODO: add description
func DefaultServerCertSANs(cfg *EtcdAdmConfig) {
	cfg.ServerCertSANs = append(cfg.ServerCertSANs, cfg.Name)

	uniqueSANs := make(map[string]int)
	for _, url := range cfg.AdvertiseClientURLs {
		uniqueSANs[url.Hostname()] = 0
	}
	for _, url := range cfg.ListenClientURLs {
		uniqueSANs[url.Hostname()] = 0
	}
	for san := range uniqueSANs {
		cfg.ServerCertSANs = append(cfg.ServerCertSANs, san)
	}
}

// DefaultPeerCertSANs TODO: add description
func DefaultPeerCertSANs(cfg *EtcdAdmConfig) {
	cfg.PeerCertSANs = append(cfg.PeerCertSANs, cfg.Name)

	uniqueSANs := make(map[string]int)
	for _, url := range cfg.InitialAdvertisePeerURLs {
		uniqueSANs[url.Hostname()] = 0
	}
	for _, url := range cfg.ListenPeerURLs {
		uniqueSANs[url.Hostname()] = 0
	}
	for san := range uniqueSANs {
		cfg.PeerCertSANs = append(cfg.PeerCertSANs, san)
	}
}

// DefaultPeerURLs TODO: add description
func DefaultPeerURLs(cfg *EtcdAdmConfig) error {
	if err := DefaultInitialAdvertisePeerURLs(cfg); err != nil {
		return err
	}
	return DefaultListenPeerURLs(cfg)
}

// DefaultClientURLs TODO: add description
func DefaultClientURLs(cfg *EtcdAdmConfig) error {
	DefaultLoopbackClientURL(cfg)
	DefaultAdvertiseClientURLs(cfg)
	DefaultListenClientURLs(cfg)
	return nil
}

// DefaultInitialAdvertisePeerURLs TODO: add description
func DefaultInitialAdvertisePeerURLs(cfg *EtcdAdmConfig) error {
	externalAddress, err := defaultExternalAddress(cfg.BindAddr)

	if err != nil {
		return fmt.Errorf("failed to set default InitialAdvertisePeerURLs: %s", err)
	}
	cfg.InitialAdvertisePeerURLs = append(cfg.InitialAdvertisePeerURLs, url.URL{
		Scheme: "https",
		Host:   fmt.Sprintf("%s:%d", externalAddress.String(), constants.DefaultPeerPort),
	})
	return nil
}

// DefaultListenPeerURLs TODO: add description
func DefaultListenPeerURLs(cfg *EtcdAdmConfig) error {
	cfg.ListenPeerURLs = cfg.InitialAdvertisePeerURLs
	return nil
}

// DefaultLoopbackClientURL TODO: add description
func DefaultLoopbackClientURL(cfg *EtcdAdmConfig) {
	cfg.LoopbackClientURL = url.URL{
		Scheme: "https",
		Host:   fmt.Sprintf("%s:%d", constants.DefaultLoopbackHost, constants.DefaultClientPort),
	}
}

// DefaultListenClientURLs TODO: add description
func DefaultListenClientURLs(cfg *EtcdAdmConfig) error {
	cfg.ListenClientURLs = cfg.AdvertiseClientURLs
	cfg.ListenClientURLs = append(cfg.ListenClientURLs, cfg.LoopbackClientURL)
	return nil
}

// DefaultAdvertiseClientURLs TODO: add description
func DefaultAdvertiseClientURLs(cfg *EtcdAdmConfig) error {
	externalAddress, err := defaultExternalAddress(cfg.BindAddr)
	if err != nil {
		return fmt.Errorf("failed to set default AdvertiseClientURLs: %s", err)
	}
	cfg.AdvertiseClientURLs = append(cfg.AdvertiseClientURLs, url.URL{
		Scheme: "https",
		Host:   fmt.Sprintf("%s:%d", externalAddress.String(), constants.DefaultClientPort),
	})
	return nil
}

// Returns the address associated with the host's default interface.
func defaultExternalAddress(addr string) (net.IP, error) {
	var (
		ip  net.IP
		err error
	)

	if addr == "" || addr == "0.0.0.0" {
		ip, err := netutil.ResolveBindAddress(net.ParseIP("0.0.0.0"))
		if err != nil {
			return nil, fmt.Errorf("failed to find a default external address: %s", err)
		}
		return ip, nil
	}

	ip = net.ParseIP(addr)
	if ip == nil {
		return nil, fmt.Errorf(`invalid bind address "%s": invalid ip format`, addr)
	}

	systemAddrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, fmt.Errorf("unable to list system unicast addresses to validate bind address: %w", err)
	}

	for _, sa := range systemAddrs {
		if ipNet, ok := sa.(*net.IPNet); ok {
			if ipNet.IP.Equal(ip) {
				return ip, nil
			}
		}
	}

	return nil, fmt.Errorf("invalid bind address %s: not found in system interfaces", addr)
}
