package apis

const (
	DefaultBindAddressv4 = "0.0.0.0"
)

type EtcdAdmConfig struct {
	Version             string
	Name                string
	InitialCluster      string
	InitialClusterToken string
	CertificatesDir     string
}
