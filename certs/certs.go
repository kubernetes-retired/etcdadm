/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

--

This is a copy of k8s.io/kubernetes/cmd/kubeadm/app/phases/certs/certs.go,
modified to work independently of kubeadm internals like the configuration.
*/

package certs

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"

	"github.com/pkg/errors"
	log "sigs.k8s.io/etcdadm/pkg/logrus"

	certutil "k8s.io/client-go/util/cert"
	"sigs.k8s.io/etcdadm/apis"
	"sigs.k8s.io/etcdadm/certs/pkiutil"
	"sigs.k8s.io/etcdadm/constants"
)

// GetDefaultCertList returns all of the certificates etcdadm requires to function.
func GetDefaultCertList() []string {
	return []string{
		constants.EtcdServerCertAndKeyBaseName,
		constants.EtcdPeerCertAndKeyBaseName,
		constants.EtcdctlClientCertAndKeyBaseName,
		constants.APIServerEtcdClientCertAndKeyBaseName,
	}
}

// RenewCSRUsingLocalCA creates CSRs using the on-disk CA and certificates
func RenewCSRUsingLocalCA(pkiDir string, name string, csrDir string) (bool, error) {
	// reads the current certificate
	cert, err := pkiutil.TryLoadCertFromDiskIgnoreExpirationDate(pkiDir, name)
	if err != nil {
		return false, err
	}

	// extract the certificate config
	cfg := certToConfig(cert)

	csr, key, err := pkiutil.NewCSRAndKey(&cfg)
	if err != nil {
		return false, errors.Wrapf(err, "failed to renew csr %s", name)
	}
	if err := writeCSRFilesIfNotExist(csrDir, name, csr, key); err != nil {
		return false, errors.Wrapf(err, "failed to write new csr %s", name)
	}
	return true, nil
}

// RenewUsingLocalCA replaces certificates with new certificates signed by the CA.
func RenewUsingLocalCA(pkiDir string, name string) (bool, error) {
	// reads the current certificate
	cert, err := pkiutil.TryLoadCertFromDiskIgnoreExpirationDate(pkiDir, name)
	if err != nil {
		return false, err
	}

	// extract the certificate config
	cfg := certToConfig(cert)

	// reads the CA
	caCert, caKey, err := loadCertificateAuthority(pkiDir, constants.EtcdCACertAndKeyBaseName)
	if err != nil {
		return false, err
	}

	// create a new certificate with the same config
	newCert, newKey, err := pkiutil.NewCertAndKey(caCert, caKey, cfg)
	if err != nil {
		return false, errors.Wrapf(err, "failed to renew certificate %s", name)
	}

	// writes the new certificate to disk
	if err := pkiutil.WriteCertAndKey(pkiDir, name, newCert, newKey); err != nil {
		return false, errors.Wrapf(err, "failed to write new certificate %s", name)
	}

	return true, nil
}

func certToConfig(cert *x509.Certificate) certutil.Config {
	return certutil.Config{
		CommonName:   cert.Subject.CommonName,
		Organization: cert.Subject.Organization,
		AltNames: certutil.AltNames{
			IPs:      cert.IPAddresses,
			DNSNames: cert.DNSNames,
		},
		Usages: cert.ExtKeyUsage,
	}
}

// CreatePKIAssets will create and write to disk all PKI assets necessary to establish the control plane.
// If the PKI assets already exists in the target folder, they are used only if evaluated equal; otherwise an error is returned.
func CreatePKIAssets(cfg *apis.EtcdAdmConfig) error {
	log.Println("[certificates] creating PKI assets")
	certActions := []func(etcdAdmConfig *apis.EtcdAdmConfig) error{
		CreateEtcdCACertAndKeyFiles,
		CreateEtcdServerCertAndKeyFiles,
		CreateEtcdPeerCertAndKeyFiles,
		CreateEtcdctlClientCertAndKeyFiles,
		CreateAPIServerEtcdClientCertAndKeyFiles,
	}

	for _, action := range certActions {
		err := action(cfg)
		if err != nil {
			return err
		}
	}

	fmt.Printf("[certificates] valid certificates and keys now exist in %q\n", cfg.CertificatesDir)

	return nil
}

// CreateEtcdCACertAndKeyFiles create a self signed etcd CA certificate and key files.
// The etcd CA and client certs are used to secure communication between etcd peers and connections to etcd from the API server.
// This is a separate CA, so that kubernetes client identities cannot connect to etcd directly or peer with the etcd cluster.
// If the etcd CA certificate and key files already exists in the target folder, they are used only if evaluated equals; otherwise an error is returned.
func CreateEtcdCACertAndKeyFiles(cfg *apis.EtcdAdmConfig) error {
	log.Print("creating a self signed etcd CA certificate and key files")
	etcdCACert, etcdCAKey, err := NewEtcdCACertAndKey()
	if err != nil {
		return err
	}

	return writeCertificateAuthorithyFilesIfNotExist(
		cfg.CertificatesDir,
		constants.EtcdCACertAndKeyBaseName,
		etcdCACert,
		etcdCAKey,
	)
}

// CreateEtcdServerCertAndKeyFiles create a new certificate and key file for etcd.
// If the etcd serving certificate and key file already exist in the target folder, they are used only if evaluated equal; otherwise an error is returned.
// It assumes the etcd CA certificate and key file exist in the CertificatesDir
func CreateEtcdServerCertAndKeyFiles(cfg *apis.EtcdAdmConfig) error {
	log.Println("creating a new server certificate and key files for etcd")
	etcdCACert, etcdCAKey, err := loadCertificateAuthority(cfg.CertificatesDir, constants.EtcdCACertAndKeyBaseName)
	if err != nil {
		return err
	}

	etcdServerCert, etcdServerKey, err := NewEtcdServerCertAndKey(cfg, etcdCACert, etcdCAKey)
	if err != nil {
		return err
	}

	return writeCertificateFilesIfNotExist(
		cfg.CertificatesDir,
		constants.EtcdServerCertAndKeyBaseName,
		etcdCACert,
		etcdServerCert,
		etcdServerKey,
	)
}

// CreateEtcdPeerCertAndKeyFiles create a new certificate and key file for etcd peering.
// If the etcd peer certificate and key file already exist in the target folder, they are used only if evaluated equal; otherwise an error is returned.
// It assumes the etcd CA certificate and key file exist in the CertificatesDir
func CreateEtcdPeerCertAndKeyFiles(cfg *apis.EtcdAdmConfig) error {
	log.Println("creating a new certificate and key files for etcd peering")
	etcdCACert, etcdCAKey, err := loadCertificateAuthority(cfg.CertificatesDir, constants.EtcdCACertAndKeyBaseName)
	if err != nil {
		return err
	}

	etcdPeerCert, etcdPeerKey, err := NewEtcdPeerCertAndKey(cfg, etcdCACert, etcdCAKey)
	if err != nil {
		return err
	}

	return writeCertificateFilesIfNotExist(
		cfg.CertificatesDir,
		constants.EtcdPeerCertAndKeyBaseName,
		etcdCACert,
		etcdPeerCert,
		etcdPeerKey,
	)
}

// CreateEtcdctlClientCertAndKeyFiles create a new client certificate for the etcdctl client.
// If the etcdctl-client certificate and key file already exist in the target folder, they are used only if evaluated equal; otherwise an error is returned.
// It assumes the etcd CA certificate and key file exist in the CertificatesDir
func CreateEtcdctlClientCertAndKeyFiles(cfg *apis.EtcdAdmConfig) error {
	log.Println("creating a new client certificate for the etcdctl")
	etcdCACert, etcdCAKey, err := loadCertificateAuthority(cfg.CertificatesDir, constants.EtcdCACertAndKeyBaseName)
	if err != nil {
		return err
	}

	commonName := fmt.Sprintf("%s-etcdctl", cfg.Name)
	organization := constants.MastersGroup
	etcdctlClientCert, etcdctlClientKey, err := NewEtcdClientCertAndKey(etcdCACert, etcdCAKey, commonName, organization)
	if err != nil {
		return err
	}

	return writeCertificateFilesIfNotExist(
		cfg.CertificatesDir,
		constants.EtcdctlClientCertAndKeyBaseName,
		etcdCACert,
		etcdctlClientCert,
		etcdctlClientKey,
	)
}

// CreateAPIServerEtcdClientCertAndKeyFiles create a new client certificate for the apiserver calling etcd
// If the apiserver-etcd-client certificate and key file already exist in the target folder, they are used only if evaluated equal; otherwise an error is returned.
// It assumes the etcd CA certificate and key file exist in the CertificatesDir
func CreateAPIServerEtcdClientCertAndKeyFiles(cfg *apis.EtcdAdmConfig) error {
	log.Println("creating a new client certificate for the apiserver calling etcd")
	etcdCACert, etcdCAKey, err := loadCertificateAuthority(cfg.CertificatesDir, constants.EtcdCACertAndKeyBaseName)
	if err != nil {
		return err
	}
	commonName := fmt.Sprintf("%s-kube-apiserver-etcd-client", cfg.Name)
	organization := constants.MastersGroup
	apiEtcdClientCert, apiEtcdClientKey, err := NewEtcdClientCertAndKey(etcdCACert, etcdCAKey, commonName, organization)
	if err != nil {
		return err
	}

	return writeCertificateFilesIfNotExist(
		cfg.CertificatesDir,
		constants.APIServerEtcdClientCertAndKeyBaseName,
		etcdCACert,
		apiEtcdClientCert,
		apiEtcdClientKey,
	)
}

// NewEtcdCACertAndKey generate a self signed etcd CA.
func NewEtcdCACertAndKey() (*x509.Certificate, *rsa.PrivateKey, error) {

	etcdCACert, etcdCAKey, err := pkiutil.NewCertificateAuthority()
	if err != nil {
		return nil, nil, fmt.Errorf("failure while generating etcd CA certificate and key: %v", err)
	}

	return etcdCACert, etcdCAKey, nil
}

// NewEtcdServerCertAndKey generate certificate for etcd, signed by the given CA.
func NewEtcdServerCertAndKey(cfg *apis.EtcdAdmConfig, caCert *x509.Certificate, caKey *rsa.PrivateKey) (*x509.Certificate, *rsa.PrivateKey, error) {

	altNames, err := pkiutil.GetEtcdAltNames(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failure while composing altnames for etcd: %v", err)
	}

	config := certutil.Config{
		CommonName: cfg.Name,
		AltNames:   *altNames,
		Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	etcdServerCert, etcdServerKey, err := pkiutil.NewCertAndKey(caCert, caKey, config)
	if err != nil {
		return nil, nil, fmt.Errorf("failure while creating etcd key and certificate: %v", err)
	}

	return etcdServerCert, etcdServerKey, nil
}

// NewEtcdPeerCertAndKey generate certificate for etcd peering, signed by the given CA.
func NewEtcdPeerCertAndKey(cfg *apis.EtcdAdmConfig, caCert *x509.Certificate, caKey *rsa.PrivateKey) (*x509.Certificate, *rsa.PrivateKey, error) {

	altNames, err := pkiutil.GetEtcdPeerAltNames(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failure while composing altnames for etcd peering: %v", err)
	}

	config := certutil.Config{
		CommonName: cfg.Name,
		AltNames:   *altNames,
		Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	etcdPeerCert, etcdPeerKey, err := pkiutil.NewCertAndKey(caCert, caKey, config)
	if err != nil {
		return nil, nil, fmt.Errorf("failure while creating etcd peer key and certificate: %v", err)
	}

	return etcdPeerCert, etcdPeerKey, nil
}

// NewEtcdClientCertAndKey generates a client certificate to connect to etcd securely, signed by the given CA.
func NewEtcdClientCertAndKey(caCert *x509.Certificate, caKey *rsa.PrivateKey, commonName string, organization string) (*x509.Certificate, *rsa.PrivateKey, error) {

	config := certutil.Config{
		CommonName:   commonName,
		Organization: []string{organization},
		Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	apiClientCert, apiClientKey, err := pkiutil.NewCertAndKey(caCert, caKey, config)
	if err != nil {
		return nil, nil, fmt.Errorf("failure while creating %q etcd client key and certificate: %v", commonName, err)
	}

	return apiClientCert, apiClientKey, nil
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

// writeCertificateAuthorithyFilesIfNotExist write a new certificate Authority to the given path.
// If there already is a certificate file at the given path; kubeadm tries to load it and check if the values in the
// existing and the expected certificate equals. If they do; kubeadm will just skip writing the file as it's up-to-date,
// otherwise this function returns an error.
func writeCertificateAuthorithyFilesIfNotExist(pkiDir string, baseName string, caCert *x509.Certificate, caKey *rsa.PrivateKey) error {

	// If cert or key exists, we should try to load them
	if pkiutil.CertOrKeyExist(pkiDir, baseName) {

		// Try to load .crt and .key from the PKI directory
		caCert, _, err := pkiutil.TryLoadCertAndKeyFromDisk(pkiDir, baseName)
		if err != nil {
			return fmt.Errorf("failure loading %s certificate: %v", baseName, err)
		}

		// Check if the existing cert is a CA
		if !caCert.IsCA {
			return fmt.Errorf("certificate %s is not a CA", baseName)
		}

		// kubeadm doesn't validate the existing certificate Authority more than this;
		// Basically, if we find a certificate file with the same path; and it is a CA
		// kubeadm thinks those files are equal and doesn't bother writing a new file
		fmt.Printf("[certificates] Using the existing %s certificate and key.\n", baseName)
	} else {

		// Write .crt and .key files to disk
		if err := pkiutil.WriteCertAndKey(pkiDir, baseName, caCert, caKey); err != nil {
			return fmt.Errorf("failure while saving %s certificate and key: %v", baseName, err)
		}

		fmt.Printf("[certificates] Generated %s certificate and key.\n", baseName)
	}
	return nil
}

// writeCertificateFilesIfNotExist write a new certificate to the given path.
// If there already is a certificate file at the given path; kubeadm tries to load it and check if the values in the
// existing and the expected certificate equals. If they do; kubeadm will just skip writing the file as it's up-to-date,
// otherwise this function returns an error.
func writeCertificateFilesIfNotExist(pkiDir string, baseName string, signingCert *x509.Certificate, cert *x509.Certificate, key *rsa.PrivateKey) error {

	// Checks if the signed certificate exists in the PKI directory
	if pkiutil.CertOrKeyExist(pkiDir, baseName) {
		// Try to load signed certificate .crt and .key from the PKI directory
		signedCert, _, err := pkiutil.TryLoadCertAndKeyFromDisk(pkiDir, baseName)
		if err != nil {
			return fmt.Errorf("failure loading %s certificate: %v", baseName, err)
		}

		// Check if the existing cert is signed by the given CA
		if err := signedCert.CheckSignatureFrom(signingCert); err != nil {
			return fmt.Errorf("certificate %s is not signed by corresponding CA", baseName)
		}

		// kubeadm doesn't validate the existing certificate more than this;
		// Basically, if we find a certificate file with the same path; and it is signed by
		// the expected certificate authority, kubeadm thinks those files are equal and
		// doesn't bother writing a new file
		fmt.Printf("[certificates] Using the existing %s certificate and key.\n", baseName)
	} else {

		// Write .crt and .key files to disk
		if err := pkiutil.WriteCertAndKey(pkiDir, baseName, cert, key); err != nil {
			return fmt.Errorf("failure while saving %s certificate and key: %v", baseName, err)
		}

		fmt.Printf("[certificates] Generated %s certificate and key.\n", baseName)
		if pkiutil.HasServerAuth(cert) {
			fmt.Printf("[certificates] %s serving cert is signed for DNS names %v and IPs %v\n", baseName, cert.DNSNames, cert.IPAddresses)
		}
	}

	return nil
}

// writeKeyFilesIfNotExist write a new key to the given path.
// If there already is a key file at the given path; kubeadm tries to load it and check if the values in the
// existing and the expected key equals. If they do; kubeadm will just skip writing the file as it's up-to-date,
// otherwise this function returns an error.
func writeKeyFilesIfNotExist(pkiDir string, baseName string, key *rsa.PrivateKey) error {

	// Checks if the key exists in the PKI directory
	if pkiutil.CertOrKeyExist(pkiDir, baseName) {

		// Try to load .key from the PKI directory
		_, err := pkiutil.TryLoadKeyFromDisk(pkiDir, baseName)
		if err != nil {
			return fmt.Errorf("%s key existed but it could not be loaded properly: %v", baseName, err)
		}

		// kubeadm doesn't validate the existing certificate key more than this;
		// Basically, if we find a key file with the same path kubeadm thinks those files
		// are equal and doesn't bother writing a new file
		fmt.Printf("[certificates] Using the existing %s key.\n", baseName)
	} else {

		// Write .key and .pub files to disk
		if err := pkiutil.WriteKey(pkiDir, baseName, key); err != nil {
			return fmt.Errorf("failure while saving %s key: %v", baseName, err)
		}

		if err := pkiutil.WritePublicKey(pkiDir, baseName, &key.PublicKey); err != nil {
			return fmt.Errorf("failure while saving %s public key: %v", baseName, err)
		}
		fmt.Printf("[certificates] Generated %s key and public key.\n", baseName)
	}

	return nil
}

func writeCSRFilesIfNotExist(csrDir string, baseName string, csr *x509.CertificateRequest, key *rsa.PrivateKey) error {
	if pkiutil.CSROrKeyExist(csrDir, baseName) {
		_, _, err := pkiutil.TryLoadCSRAndKeyFromDisk(csrDir, baseName)
		if err != nil {
			return errors.Wrapf(err, "%s CSR existed but it could not be loaded properly", baseName)
		}

		fmt.Printf("[certs] Using the existing %q CSR\n", baseName)
	} else {
		// Write .key and .csr files to disk
		fmt.Printf("[certs] Generating %q key and CSR\n", baseName)

		if err := pkiutil.WriteKey(csrDir, baseName, key); err != nil {
			return errors.Wrapf(err, "failure while saving %s key", baseName)
		}

		if err := pkiutil.WriteCSR(csrDir, baseName, csr); err != nil {
			return errors.Wrapf(err, "failure while saving %s CSR", baseName)
		}
	}

	return nil
}
