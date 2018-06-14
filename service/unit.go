package service

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"runtime"

	"github.com/platform9/etcdadm/apis"
	"github.com/platform9/etcdadm/constants"
	netutil "k8s.io/apimachinery/pkg/util/net"
)

const (
	unitFile        = "/etc/systemd/system/etcd.service"
	environmentFile = "/etc/etcd.env"
)

const (
	unitFileTemplate = `[Unit]
Description=etcd
Documentation=https://github.com/coreos/etcd
Conflicts=etcd-member.service
Conflicts=etcd2.service

[Service]
EnvironmentFile={{ .EnvironmentFile }}
ExecStart={{ .Executable }}
Type=notify
Restart=on-failure
RestartSec=5s
TimeoutStartSec=0

LimitNOFILE=65536
Nice=-10
IOSchedulingClass=best-effort
IOSchedulingPriority=2

[Install]
WantedBy=multi-user.target`

	envFileTemplate = `
ETCD_NAME={{ .Name }}

ETCD_DATA_DIR=/var/lib/etcd

ETCD_ADVERTISE_CLIENT_URLS=https://localhost:2379
ETCD_LISTEN_CLIENT_URLS=https://localhost:2379

ETCD_LISTEN_PEER_URLS=https://{{ .IP }}:2380
ETCD_INITIAL_ADVERTISE_PEER_URLS=https://{{ .IP }}:2380

ETCD_CERT_FILE={{ .CertificatesDir }}/server.pem
ETCD_KEY_FILE={{ .CertificatesDir }}/server-key.pem
ETCD_TRUSTED_CA_FILE={{ .CertificatesDir }}/ca.pem
ETCD_CLIENT_CERT_AUTH=true

ETCD_PEER_KEY_FILE={{ .CertificatesDir }}/peer.pem
ETCD_PEER_CERT_FILE={{ .CertificatesDir }}/peer-key.pem
ETCD_PEER_TRUSTED_CA_FILE={{ .CertificatesDir }}/ca.pem
ETCD_PEER_CLIENT_CERT_AUTH=true

ETCD_INITIAL_CLUSTER={{ .InitialCluster }}
ETCD_INITIAL_CLUSTER_TOKEN={{ .InitialClusterToken }}
ETCD_INITIAL_CLUSTER_STATE={{ .InitialClusterState }}
ETCD_STRICT_RECONFIG_CHECK=true

GOMAXPROCS={{ .GOMAXPROCS }}`
)

// etcdEnvironment is used to set the environment of the etcd service
type etcdEnvironment struct {
	Name                string
	InitialCluster      string
	InitialClusterToken string
	InitialClusterState string
	IP                  string
	CertificatesDir     string
	GOMAXPROCS          int
}

type etcdUnit struct {
	EnvironmentFile string
	Executable      string
}

func newEnvironment(etcdAdmConfig *apis.EtcdAdmConfig) (*etcdEnvironment, error) {
	env := &etcdEnvironment{
		Name:                etcdAdmConfig.Name,
		InitialCluster:      etcdAdmConfig.InitialCluster,
		InitialClusterToken: etcdAdmConfig.InitialClusterToken,
		InitialClusterState: etcdAdmConfig.InitialClusterState,
		CertificatesDir:     etcdAdmConfig.CertificatesDir,
		GOMAXPROCS:          runtime.NumCPU(),
	}

	ip, err := netutil.ChooseHostInterface()
	if err != nil {
		return nil, err
	}
	env.IP = ip.String()
	if ip.To4() != nil {
		env.IP = constants.DefaultBindAddressv4
	}

	return env, nil
}

// WriteEnvironmentFile writes the environment file used by the etcd service unit
func WriteEnvironmentFile(etcdAdmConfig *apis.EtcdAdmConfig) error {
	t := template.Must(template.New("environment").Parse(envFileTemplate))

	f, err := os.OpenFile(environmentFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("unable to open the etcd environment file %s: %s", environmentFile, err)
	}
	defer f.Close()

	env, err := newEnvironment(etcdAdmConfig)
	if err := t.Execute(f, env); err != nil {
		return fmt.Errorf("unable to apply the etcd environment: %s", err)
	}
	return nil
}

// WriteUnitFile writes etcd service unit file
func WriteUnitFile(etcdAdmConfig *apis.EtcdAdmConfig) error {
	t := template.Must(template.New("unit").Parse(unitFileTemplate))

	f, err := os.OpenFile(unitFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("unable to open the etcd service unit file %s: %s", unitFile, err)
	}
	defer f.Close()

	unit := &etcdUnit{
		EnvironmentFile: environmentFile,
		Executable:      filepath.Join(etcdAdmConfig.CertificatesDir, "etcd"),
	}
	if err := t.Execute(f, unit); err != nil {
		return fmt.Errorf("unable to apply etcd environment: %s", err)
	}
	return nil
}
