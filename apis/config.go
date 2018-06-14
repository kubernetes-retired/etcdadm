package apis

// EtcdAdmConfig holds etcdadm configuration
type EtcdAdmConfig struct {
	Version             string
	ReleaseURL          string
	Name                string
	InitialCluster      string
	InitialClusterToken string
	InstallDir          string
	CertificatesDir     string
}
