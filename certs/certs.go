package certs

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"

	"github.com/platform9/etcdadm/apis"
	"github.com/platform9/etcdadm/certs/pkiutil"
)

// CreateAPIServerCertAndKeyFiles create a new certificate and key files for etcd.
// If the etcd certificate and key files already exists in the target folder, they are used only if evaluated equal; otherwise an error is returned.
// It assumes the cluster CA certificate and key files exist in the CertificatesDir.
func CreateAPIServerCertAndKeyFiles(cfg *apis.EtcdAdmConfig) error {
	caCert, caKey, err := loadCertificateAuthority(cfg.CertificatesDir, kubeadmconstants.CACertAndKeyBaseName)
	if err != nil {
		return err
	}

	apiCert, apiKey, err := NewAPIServerCertAndKey(cfg, caCert, caKey)
	if err != nil {
		return err
	}

	return writeCertificateFilesIfNotExist(
		cfg.CertificatesDir,
		kubeadmconstants.APIServerCertAndKeyBaseName,
		caCert,
		apiCert,
		apiKey,
	)
}

// loadCertificateAuthority loads certificate authority
func loadCertificateAuthority(pkiDir string, baseName string) (*x509.Certificate, *rsa.PrivateKey, error) {
	// Checks if certificate authority exists in the PKI directory
	if !pkiutil.CertOrKeyExist(pkiDir, baseName) {
		return nil, nil, fmt.Errorf("couldn't load %s certificate authority from %s", baseName, pkiDir)
	}

	// Try to load certificate authority .crt and .key from the PKI directory
	caCert, caKey, err := pkiutil.TryLoadCertAndKeyFromDisk(pkiDir, baseName)
	if err != nil {
		return nil, nil, fmt.Errorf("failure loading %s certificate authority: %v", baseName, err)
	}

	// Make sure the loaded CA cert actually is a CA
	if !caCert.IsCA {
		return nil, nil, fmt.Errorf("%s certificate is not a certificate authority", baseName)
	}

	return caCert, caKey, nil
}
