package constants

import (
	"fmt"
	"path/filepath"
)

// Command-line flag defaults
const (
	DefaultVersion        = "3.1.12"
	DefaultInstallBaseDir = "/opt/bin/"
	DefaultReleaseURL     = "https://github.com/coreos/etcd/releases/download"
	DefaultBindAddressv4  = "0.0.0.0"
	DefaultCertificateDir = "/etc/etcd/pki"

	Unit            = "etcd.service"
	UnitFile        = "/etc/systemd/system/etcd.service"
	EnvironmentFile = "/etc/etcd.env"

	// DefaultName defines the default etcd member name. It is left blank to let etcd choose the default.
	DefaultName = ""

	// DefaultName defines the default etcd cluster token. It is left blank to let etcd choose the default.
	DefaultInitialClusterToken = ""
	DefaultInitialCluster      = ""
)

func DefaultInstallDir() string {
	return filepath.Join(DefaultInstallBaseDir, fmt.Sprintf("etcd-v%s", DefaultVersion))
}
