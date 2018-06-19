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

	UnitFile        string
	EnvironmentFile string
	EtcdExecutable  string

	EtcdctlEnvFile string

	AdvertisePeerURLs   URLList
	ListenPeerURLs      URLList
	AdvertiseClientURLs URLList
	ListenClientURLs    URLList

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
}

type URLList []url.URL

func (l URLList) String() string {
	stringURLs := make([]string, len(l))
	for i, url := range l {
		stringURLs[i] = url.String()
	}
	return strings.Join(stringURLs, ",")
}

// SetInitDynamicDefaults checks and sets configuration values used by the init verb
func SetInitDynamicDefaults(cfg *EtcdAdmConfig) error {
	cfg.InitialClusterState = "new"
	if err := setDynamicDefaults(cfg); err != nil {
		return err
	}
	InitialClusterInit(cfg)
	return nil
}

// SetJoinDynamicDefaults checks and sets configuration values used by the join verb
func SetJoinDynamicDefaults(cfg *EtcdAdmConfig) error {
	cfg.InitialClusterState = "existing"
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
	cfg.DataDir = constants.DefaultDataDir
	cfg.UnitFile = constants.UnitFile
	cfg.EnvironmentFile = constants.EnvironmentFile
	cfg.EtcdExecutable = filepath.Join(cfg.InstallDir, "etcd")
	cfg.EtcdctlEnvFile = constants.EtcdctlEnvFile

	cfg.GOMAXPROCS = runtime.NumCPU()

	cfg.InitialClusterToken = uuid.NewV4().String()[0:8]
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

func DefaultListenClientURLs(cfg *EtcdAdmConfig) error {
	cfg.ListenClientURLs = cfg.AdvertiseClientURLs
	cfg.ListenClientURLs = append(cfg.ListenClientURLs, url.URL{
		Scheme: "https",
		Host:   fmt.Sprintf("%s:%d", constants.DefaultLoopbackHost, constants.DefaultClientPort),
	})
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
