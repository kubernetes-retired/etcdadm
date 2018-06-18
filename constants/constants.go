package constants

// Command-line flag defaults
const (
	DefaultVersion        = "3.1.12"
	DefaultInstallBaseDir = "/opt/bin/"

	DefaultReleaseURL     = "https://github.com/coreos/etcd/releases/download"
	DefaultBindAddressv4  = "0.0.0.0"
	DefaultCertificateDir = "/etc/etcd/pki"

	UnitFile        = "/etc/systemd/system/etcd.service"
	EnvironmentFile = "/etc/etcd/etcd.env"

	DefaultDataDir = "/var/lib/etcd"

	DefaultLoopbackHost = "127.0.0.1"
	DefaultPeerPort     = 2380
	DefaultClientPort   = 2379

	// EtcdCACertAndKeyBaseName defines etcd's CA certificate and key base name
	EtcdCACertAndKeyBaseName = "etcd/ca"
	// EtcdCACertName defines etcd's CA certificate name
	EtcdCACertName = "etcd/ca.crt"
	// EtcdCAKeyName defines etcd's CA key name
	EtcdCAKeyName = "etcd/ca.key"

	// EtcdServerCertAndKeyBaseName defines etcd's server certificate and key base name
	EtcdServerCertAndKeyBaseName = "etcd/server"
	// EtcdServerCertName defines etcd's server certificate name
	EtcdServerCertName = "etcd/server.crt"
	// EtcdServerKeyName defines etcd's server key name
	EtcdServerKeyName = "etcd/server.key"

	// EtcdPeerCertAndKeyBaseName defines etcd's peer certificate and key base name
	EtcdPeerCertAndKeyBaseName = "etcd/peer"
	// EtcdPeerCertName defines etcd's peer certificate name
	EtcdPeerCertName = "etcd/peer.crt"
	// EtcdPeerKeyName defines etcd's peer key name
	EtcdPeerKeyName = "etcd/peer.key"

	// APIServerEtcdClientCertAndKeyBaseName defines apiserver's etcd client certificate and key base name
	APIServerEtcdClientCertAndKeyBaseName = "apiserver-etcd-client"
	// APIServerEtcdClientCertName defines apiserver's etcd client certificate name
	APIServerEtcdClientCertName = "apiserver-etcd-client.crt"
	// APIServerEtcdClientKeyName defines apiserver's etcd client key name
	APIServerEtcdClientKeyName = "apiserver-etcd-client.key"
	// APIServerEtcdClientCertCommonName defines apiserver's etcd client certificate common name (CN)
	APIServerEtcdClientCertCommonName = "kube-apiserver-etcd-client"

	// MastersGroup defines the well-known group for the apiservers. This group is also superuser by default
	// (i.e. bound to the cluster-admin ClusterRole)
	MastersGroup = "system:masters"

	UnitFileTemplate = `[Unit]
Description=etcd
Documentation=https://github.com/coreos/etcd
Conflicts=etcd-member.service
Conflicts=etcd2.service

[Service]
EnvironmentFile={{ .EnvironmentFile }}
ExecStart={{ .EtcdExecutable }}

Type=notify
TimeoutStartSec=0
Restart=on-failure
RestartSec=5s

LimitNOFILE=65536
Nice=-10
IOSchedulingClass=best-effort
IOSchedulingPriority=2

[Install]
WantedBy=multi-user.target
`

	EnvFileTemplate = `ETCD_NAME={{ .Name }}

ETCD_INITIAL_CLUSTER={{ .InitialCluster }}
ETCD_INITIAL_CLUSTER_TOKEN={{ .InitialClusterToken }}
ETCD_INITIAL_CLUSTER_STATE={{ .InitialClusterState }}

ETCD_ADVERTISE_CLIENT_URLS={{ .AdvertiseClientURLs }}
ETCD_LISTEN_CLIENT_URLS={{ .ListenClientURLs }}

ETCD_INITIAL_ADVERTISE_PEER_URLS={{ .AdvertisePeerURLs }}
ETCD_LISTEN_PEER_URLS={{ .ListenPeerURLs }}

ETCD_CERT_FILE={{ .CertificatesDir }}/server.crt
ETCD_KEY_FILE={{ .CertificatesDir }}/server.key
ETCD_TRUSTED_CA_FILE={{ .CertificatesDir }}/ca.crt

ETCD_PEER_KEY_FILE={{ .CertificatesDir }}/peer.crt
ETCD_PEER_CERT_FILE={{ .CertificatesDir }}/peer.key
ETCD_PEER_TRUSTED_CA_FILE={{ .CertificatesDir }}/ca.crt

ETCD_DATA_DIR={{ .DataDir }}

ETCD_STRICT_RECONFIG_CHECK=true
ETCD_CLIENT_CERT_AUTH=true
ETCD_PEER_CLIENT_CERT_AUTH=true

GOMAXPROCS={{ .GOMAXPROCS }}
`
)
