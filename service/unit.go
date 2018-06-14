package service

import (
	"fmt"
	"html/template"
	"os"

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
EnvironmentFile={{ .environmentFile }}
Type=notify
Restart=always
RestartSec=5s
LimitNOFILE=40000
TimeoutStartSec=0

ExecStart=/usr/local/bin/etcd

[Install]
WantedBy=multi-user.target`

	envFileTemplate = `
ETCD_NAME={{ .Name }}
ETCD_DATA_DIR=/var/lib/etcd
ETCD_ADVERTISE_CLIENT_URLS=https://localhost:2379
ETCD_LISTEN_CLIENT_URLS=https://localhost:2379
ETCD_LISTEN_PEER_URLS=https://{{ .IP }}:2380
ETCD_INITIAL_ADVERTISE_PEER_URLS=https://{{ .IP }}:2380
ETCD_CERT_FILE=/etc/kubernetes/pki/etcd/server.pem
ETCD_KEY_FILE=/etc/kubernetes/pki/etcd/server-key.pem
ETCD_CLIENT_CERT_AUTH=true
ETCD_TRUSTED_CA_FILE=/etc/kubernetes/pki/etcd/ca.pem
ETCD_PEER_KEY_FILE=/etc/kubernetes/pki/etcd/peer.pem
ETCD_PEER_CERT_FILE=/etc/kubernetes/pki/etcd/peer-key.pem
ETCD_PEER_CLIENT_CERT_AUTH=true
ETCD_PEER_TRUSTED_CA_FILE=/etc/kubernetes/pki/etcd/ca.pem
ETCD_INITIAL_CLUSTER={{ .InitialCluster }}
ETCD_INITIAL_CLUSTER_TOKEN={{ .InitialClusterToken }}
ETCD_INITIAL_CLUSTER_STATE={{ .InitialClusterState }}
ETCD_STRICT_RECONFIG_CHECK=true`
)

// etcdEnvironment is used to set the environment of the etcd service
type etcdEnvironment struct {
	Name                string
	InitialCluster      string
	InitialClusterToken string
	InitialClusterState string
	IP                  string
}

type etcdUnit struct {
	EnvironmentFile string
}

func newEnvironment(c *apis.EtcdAdmConfig) (*etcdEnvironment, error) {
	env := &etcdEnvironment{
		Name:                c.Name,
		InitialCluster:      c.InitialCluster,
		InitialClusterToken: c.InitialClusterToken,
		InitialClusterState: "new",
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
	}
	if err := t.Execute(f, unit); err != nil {
		return fmt.Errorf("unable to apply etcd environment: %s", err)
	}
	return nil
}
