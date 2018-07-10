package apis

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/platform9/etcdadm/constants"
	uuid "github.com/satori/go.uuid"

	netutil "k8s.io/apimachinery/pkg/util/net"
)

// EtcdAdmConfig holds etcdadm configuration
type EtcdAdmConfig struct {
	Version         string
	ReleaseURL      string
	InstallBaseDir  string
	CertificatesDir string

	DataDir    string
	InstallDir string
	CacheDir   string

	UnitFile        string
	EnvironmentFile string
	EtcdExecutable  string

	EtcdctlEnvFile string

	AdvertisePeerURLs   URLList
	ListenPeerURLs      URLList
	AdvertiseClientURLs URLList
	ListenClientURLs    URLList

	LoopbackClientURL url.URL

	// ServerCertSANs sets extra Subject Alternative Names for the etcd server signing cert.
	ServerCertSANs []string
	// PeerCertSANs sets extra Subject Alternative Names for the etcd peer signing cert.
	PeerCertSANs []string

	Name                           string
	InitialCluster                 string
	InitialClusterTokenDeclaration string
	InitialClusterState            string

	// GOMAXPROCS sets the max num of etcd processes will use
	GOMAXPROCS int
}

type EndpointStatus struct {
	EtcdMember
	// TODO(dlipovetsky) See https://github.com/coreos/etcd/blob/52ae578922ee3d63b23c797b61beba041573ce1a/etcdctl/ctlv3/command/ep_command.go#L117
	// Health string `json:"health,omitempty"`
}
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
type URLList []url.URL

func (l URLList) String() string {
	stringURLs := make([]string, len(l))
	for i, url := range l {
		stringURLs[i] = url.String()
	}
	return strings.Join(stringURLs, ",")
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
	cfg.InitialClusterTokenDeclaration = fmt.Sprintf("ETCD_INITIAL_CLUSTER_TOKEN=%s", uuid.NewV4().String()[0:8])
	InitialClusterInit(cfg)
	return nil
}

// SetJoinDynamicDefaults checks and sets configuration values used by the join verb
func SetJoinDynamicDefaults(cfg *EtcdAdmConfig) error {
	cfg.InitialClusterState = "existing"
	cfg.InitialClusterTokenDeclaration = ""
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

func setDynamicDefaults(cfg *EtcdAdmConfig) error {
	if len(cfg.Name) == 0 {
		name, err := os.Hostname()
		if err != nil {
			return fmt.Errorf("unable to use hostname as default name: %s", err)
		}
		cfg.Name = name
	}

	cfg.InstallDir = filepath.Join(cfg.InstallBaseDir, fmt.Sprintf("etcd-v%s", cfg.Version))
	cfg.CacheDir = filepath.Join(constants.DefaultCacheBaseDir, fmt.Sprintf("etcd-v%s", cfg.Version))
	cfg.DataDir = constants.DefaultDataDir
	cfg.UnitFile = constants.UnitFile
	cfg.EnvironmentFile = constants.EnvironmentFile
	cfg.EtcdExecutable = filepath.Join(cfg.InstallDir, "etcd")
	cfg.EtcdctlEnvFile = constants.EtcdctlEnvFile

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

func InitialClusterInit(cfg *EtcdAdmConfig) string {
	return fmt.Sprintf("%s=%s", cfg.Name, cfg.AdvertisePeerURLs)
}

func DefaultServerCertSANs(cfg *EtcdAdmConfig) {
	cfg.ServerCertSANs = append(cfg.ServerCertSANs, cfg.Name)

	uniqueSANs := make(map[string]int)
	for _, url := range cfg.AdvertiseClientURLs {
		uniqueSANs[url.Hostname()] = 0
	}
	for _, url := range cfg.ListenClientURLs {
		uniqueSANs[url.Hostname()] = 0
	}
	for san, _ := range uniqueSANs {
		cfg.ServerCertSANs = append(cfg.ServerCertSANs, san)
	}
}

func DefaultPeerCertSANs(cfg *EtcdAdmConfig) {
	cfg.PeerCertSANs = append(cfg.PeerCertSANs, cfg.Name)

	uniqueSANs := make(map[string]int)
	for _, url := range cfg.AdvertisePeerURLs {
		uniqueSANs[url.Hostname()] = 0
	}
	for _, url := range cfg.ListenPeerURLs {
		uniqueSANs[url.Hostname()] = 0
	}
	for san, _ := range uniqueSANs {
		cfg.PeerCertSANs = append(cfg.PeerCertSANs, san)
	}
}

func DefaultPeerURLs(cfg *EtcdAdmConfig) error {
	if err := DefaultAdvertisePeerURLs(cfg); err != nil {
		return err
	}
	return DefaultListenPeerURLs(cfg)
}

func DefaultClientURLs(cfg *EtcdAdmConfig) error {
	DefaultLoopbackClientURL(cfg)
	DefaultAdvertiseClientURLs(cfg)
	DefaultListenClientURLs(cfg)
	return nil
}

func DefaultAdvertisePeerURLs(cfg *EtcdAdmConfig) error {
	externalAddress, err := defaultExternalAddress()
	if err != nil {
		return fmt.Errorf("failed to set default AdvertisePeerURLs: %s", err)
	}
	cfg.AdvertisePeerURLs = append(cfg.AdvertisePeerURLs, url.URL{
		Scheme: "https",
		Host:   fmt.Sprintf("%s:%d", externalAddress.String(), constants.DefaultPeerPort),
	})
	return nil
}

func DefaultListenPeerURLs(cfg *EtcdAdmConfig) error {
	cfg.ListenPeerURLs = cfg.AdvertisePeerURLs
	return nil
}

func DefaultLoopbackClientURL(cfg *EtcdAdmConfig) {
	cfg.LoopbackClientURL = url.URL{
		Scheme: "https",
		Host:   fmt.Sprintf("%s:%d", constants.DefaultLoopbackHost, constants.DefaultClientPort),
	}
}

func DefaultListenClientURLs(cfg *EtcdAdmConfig) error {
	cfg.ListenClientURLs = cfg.AdvertiseClientURLs
	cfg.ListenClientURLs = append(cfg.ListenClientURLs, cfg.LoopbackClientURL)
	return nil
}

func DefaultAdvertiseClientURLs(cfg *EtcdAdmConfig) error {
	externalAddress, err := defaultExternalAddress()
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
func defaultExternalAddress() (net.IP, error) {
	ip, err := netutil.ChooseBindAddress(net.ParseIP("0.0.0.0"))
	if err != nil {
		return nil, fmt.Errorf("failed to find a default external address: %s", err)
	}
	return ip, nil
}
