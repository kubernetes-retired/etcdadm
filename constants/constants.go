package constants

import (
	"fmt"
	"path/filepath"
)

const (
	DefaultVersion        = "3.1.12"
	DefaultInstallBaseDir = "/opt/bin/"
	DefaultReleaseURL     = "https://github.com/coreos/etcd/releases/download"
	DefaultBindAddressv4  = "0.0.0.0"
	DefaultCertificateDir = "/etc/etcd/pki"
)

func DefaultInstallDir() string {
	return filepath.Join(DefaultInstallBaseDir, fmt.Sprintf("etcd-v%s", DefaultVersion))
}

func ReleaseFile(version string) string {
	return fmt.Sprintf("etcd-v%s-linux-amd64.tar.gz", version)
}

func DownloadURL(releaseURL, version string) string {
	return fmt.Sprintf("%s/v%s/%s", releaseURL, version, ReleaseFile(version))
}
