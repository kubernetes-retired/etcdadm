package apis

import (
	"fmt"
	"net"
	"net/url"
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

	UnitFile        string
	EnvironmentFile string
	EtcdExecutable  string

	AdvertisePeerURLs   string
	ListenPeerURLs      string
	AdvertiseClientURLs string
	ListenClientURLs    string

	Name                string
	InitialCluster      string
	InitialClusterToken string
	InitialClusterState string

	// ServerCertSANs sets extra Subject Alternative Names for the etcd server signing cert.
	ServerCertSANs []string
	// PeerCertSANs sets extra Subject Alternative Names for the etcd peer signing cert.
	PeerCertSANs []string

	// GOMAXPROCS sets the max num of etcd processes will use
	GOMAXPROCS int
}

// SetInitDynamicDefaults checks and sets configuration values used by the init verb
func SetInitDynamicDefaults(cfg *EtcdAdmConfig) error {
	cfg.InitialClusterState = "new"
	return setDynamicDefaults(cfg)
}

// SetJoinDynamicDefaults checks and sets configuration values used by the join verb
func SetJoinDynamicDefaults(cfg *EtcdAdmConfig) error {
	cfg.InitialClusterState = "existing"
	return setDynamicDefaults(cfg)
}

func setDynamicDefaults(cfg *EtcdAdmConfig) error {
	cfg.InstallDir = filepath.Join(cfg.InstallBaseDir, fmt.Sprintf("etcd-v%s", cfg.Version))
	cfg.DataDir = constants.DefaultDataDir
	cfg.UnitFile = constants.UnitFile
	cfg.EnvironmentFile = constants.EnvironmentFile
	cfg.EtcdExecutable = filepath.Join(cfg.InstallDir, "etcd")

	cfg.GOMAXPROCS = runtime.NumCPU()

	cfg.InitialClusterToken = uuid.NewV4().String()[0:8]
	if err := DefaultPeerURLs(cfg); err != nil {
		return err
	}
	if err := DefaultClientURLs(cfg); err != nil {
		return err
	}
	cfg.InitialCluster = InitialCluster(cfg)
	return nil
}

func InitialCluster(cfg *EtcdAdmConfig) string {
	return fmt.Sprintf("%s=%s", cfg.Name, cfg.AdvertisePeerURLs)
}

func DefaultPeerURLs(cfg *EtcdAdmConfig) error {
	if err := DefaultAdvertisePeerURLs(cfg); err != nil {
		return err
	}
	return DefaultListenPeerURLs(cfg)
}

func DefaultClientURLs(cfg *EtcdAdmConfig) error {
	DefaultAdvertiseClientURLs(cfg)
	DefaultListenClientURLs(cfg)
	return nil
}

func DefaultAdvertisePeerURLs(cfg *EtcdAdmConfig) error {
	externalAddress, err := defaultExternalAddress()
	if err != nil {
		return fmt.Errorf("failed to set default AdvertisePeerURLs: %s", err)
	}
	urls := make([]url.URL, 1)
	urls[0] = url.URL{
		Scheme: "https",
		Host:   fmt.Sprintf("%s:%d", externalAddress.String(), constants.DefaultPeerPort),
	}
	cfg.AdvertisePeerURLs = urls[0].String()
	return nil
}

func DefaultListenPeerURLs(cfg *EtcdAdmConfig) error {
	cfg.ListenPeerURLs = cfg.AdvertisePeerURLs
	return nil
}

func DefaultListenClientURLs(cfg *EtcdAdmConfig) error {
	url := url.URL{
		Scheme: "https",
		Host:   fmt.Sprintf("%s:%d", constants.DefaultLoopbackHost, constants.DefaultClientPort),
	}
	cfg.ListenClientURLs = strings.Join([]string{
		cfg.AdvertiseClientURLs,
		url.String(),
	}, ",")
	return nil
}

func DefaultAdvertiseClientURLs(cfg *EtcdAdmConfig) error {
	externalAddress, err := defaultExternalAddress()
	if err != nil {
		return fmt.Errorf("failed to set default AdvertiseClientURLs: %s", err)
	}
	urls := make([]url.URL, 1)
	urls[0] = url.URL{
		Scheme: "https",
		Host:   fmt.Sprintf("%s:%d", externalAddress.String(), constants.DefaultClientPort),
	}
	cfg.AdvertiseClientURLs = urls[0].String()
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
