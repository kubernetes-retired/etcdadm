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

This is a copy of k8s.io/kubernetes/cmd/kubeadm/app/phases/certs/pkiutil/pki_helpers.go
modified to work independently of kubeadm internals like the configuration.
*/

package pkiutil

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"k8s.io/apimachinery/pkg/util/validation"
	certutil "k8s.io/client-go/util/cert"
	"sigs.k8s.io/etcdadm/apis"
	"sigs.k8s.io/etcdadm/constants"

	log "sigs.k8s.io/etcdadm/pkg/logrus"
)

// NewCertificateAuthority creates new certificate and private key for the certificate authority
func NewCertificateAuthority() (*x509.Certificate, *rsa.PrivateKey, error) {
	key, err := certutil.NewPrivateKey()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create private key [%v]", err)
	}

	config := certutil.Config{
		CommonName: "etcd",
	}
	cert, err := certutil.NewSelfSignedCACert(config, key)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create self-signed certificate [%v]", err)
	}

	return cert, key, nil
}

// NewCertAndKey creates new certificate and key by passing the certificate authority certificate and key
func NewCertAndKey(caCert *x509.Certificate, caKey *rsa.PrivateKey, config certutil.Config) (*x509.Certificate, *rsa.PrivateKey, error) {
	key, err := certutil.NewPrivateKey()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create private key [%v]", err)
	}

	cert, err := certutil.NewSignedCert(config, key, caCert, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to sign certificate [%v]", err)
	}

	return cert, key, nil
}

// HasServerAuth returns true if the given certificate is a ServerAuth
func HasServerAuth(cert *x509.Certificate) bool {
	for i := range cert.ExtKeyUsage {
		if cert.ExtKeyUsage[i] == x509.ExtKeyUsageServerAuth {
			return true
		}
	}
	return false
}

// WriteCertAndKey stores certificate and key at the specified location
func WriteCertAndKey(pkiPath string, name string, cert *x509.Certificate, key *rsa.PrivateKey) error {
	if err := WriteKey(pkiPath, name, key); err != nil {
		return err
	}

	return WriteCert(pkiPath, name, cert)
}

// WriteCert stores the given certificate at the given location
func WriteCert(pkiPath, name string, cert *x509.Certificate) error {
	if cert == nil {
		return fmt.Errorf("certificate cannot be nil when writing to file")
	}

	certificatePath := pathForCert(pkiPath, name)
	if err := certutil.WriteCert(certificatePath, certutil.EncodeCertPEM(cert)); err != nil {
		return fmt.Errorf("unable to write certificate to file %q: [%v]", certificatePath, err)
	}

	return nil
}

// WriteKey stores the given key at the given location
func WriteKey(pkiPath, name string, key *rsa.PrivateKey) error {
	if key == nil {
		return fmt.Errorf("private key cannot be nil when writing to file")
	}

	privateKeyPath := pathForKey(pkiPath, name)
	if err := certutil.WriteKey(privateKeyPath, certutil.EncodePrivateKeyPEM(key)); err != nil {
		return fmt.Errorf("unable to write private key to file %q: [%v]", privateKeyPath, err)
	}

	return nil
}

// WritePublicKey stores the given public key at the given location
func WritePublicKey(pkiPath, name string, key *rsa.PublicKey) error {
	if key == nil {
		return fmt.Errorf("public key cannot be nil when writing to file")
	}

	publicKeyBytes, err := certutil.EncodePublicKeyPEM(key)
	if err != nil {
		return err
	}
	publicKeyPath := pathForPublicKey(pkiPath, name)
	if err := certutil.WriteKey(publicKeyPath, publicKeyBytes); err != nil {
		return fmt.Errorf("unable to write public key to file %q: [%v]", publicKeyPath, err)
	}

	return nil
}

// CertOrKeyExist returns a boolean whether the cert or the key exists
func CertOrKeyExist(pkiPath, name string) bool {
	certificatePath, privateKeyPath := pathsForCertAndKey(pkiPath, name)

	_, certErr := os.Stat(certificatePath)
	_, keyErr := os.Stat(privateKeyPath)
	if os.IsNotExist(certErr) && os.IsNotExist(keyErr) {
		// The cert or the key did not exist
		return false
	}

	// Both files exist or one of them
	return true
}

// TryLoadCertAndKeyFromDisk tries to load a cert and a key from the disk and validates that they are valid
func TryLoadCertAndKeyFromDisk(pkiPath, name string) (*x509.Certificate, *rsa.PrivateKey, error) {
	cert, err := TryLoadCertFromDisk(pkiPath, name)
	if err != nil {
		return nil, nil, err
	}

	key, err := TryLoadKeyFromDisk(pkiPath, name)
	if err != nil {
		return nil, nil, err
	}

	return cert, key, nil
}

// TryLoadCertFromDisk tries to load the cert from the disk and validates that it is valid
func TryLoadCertFromDisk(pkiPath, name string) (*x509.Certificate, error) {
	certificatePath := pathForCert(pkiPath, name)

	certs, err := certutil.CertsFromFile(certificatePath)
	if err != nil {
		return nil, fmt.Errorf("couldn't load the certificate file %s: %v", certificatePath, err)
	}

	// We are only putting one certificate in the certificate pem file, so it's safe to just pick the first one
	// TODO: Support multiple certs here in order to be able to rotate certs
	cert := certs[0]

	// Check so that the certificate is valid now
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return nil, fmt.Errorf("the certificate is not valid yet")
	}
	if now.After(cert.NotAfter) {
		return nil, fmt.Errorf("the certificate has expired")
	}

	return cert, nil
}

// TryLoadKeyFromDisk tries to load the key from the disk and validates that it is valid
func TryLoadKeyFromDisk(pkiPath, name string) (*rsa.PrivateKey, error) {
	privateKeyPath := pathForKey(pkiPath, name)

	// Parse the private key from a file
	privKey, err := certutil.PrivateKeyFromFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("couldn't load the private key file %s: %v", privateKeyPath, err)
	}

	// Allow RSA format only
	var key *rsa.PrivateKey
	switch k := privKey.(type) {
	case *rsa.PrivateKey:
		key = k
	default:
		return nil, fmt.Errorf("the private key file %s isn't in RSA format", privateKeyPath)
	}

	return key, nil
}

// TryLoadPrivatePublicKeyFromDisk tries to load the key from the disk and validates that it is valid
func TryLoadPrivatePublicKeyFromDisk(pkiPath, name string) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privateKeyPath := pathForKey(pkiPath, name)

	// Parse the private key from a file
	privKey, err := certutil.PrivateKeyFromFile(privateKeyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't load the private key file %s: %v", privateKeyPath, err)
	}

	publicKeyPath := pathForPublicKey(pkiPath, name)

	// Parse the public key from a file
	pubKeys, err := certutil.PublicKeysFromFile(publicKeyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't load the public key file %s: %v", publicKeyPath, err)
	}

	// Allow RSA format only
	k, ok := privKey.(*rsa.PrivateKey)
	if !ok {
		return nil, nil, fmt.Errorf("the private key file %s isn't in RSA format", privateKeyPath)
	}

	p := pubKeys[0].(*rsa.PublicKey)

	return k, p, nil
}

func pathsForCertAndKey(pkiPath, name string) (string, string) {
	return pathForCert(pkiPath, name), pathForKey(pkiPath, name)
}

func pathForCert(pkiPath, name string) string {
	return filepath.Join(pkiPath, fmt.Sprintf("%s.crt", name))
}

func pathForKey(pkiPath, name string) string {
	return filepath.Join(pkiPath, fmt.Sprintf("%s.key", name))
}

func pathForPublicKey(pkiPath, name string) string {
	return filepath.Join(pkiPath, fmt.Sprintf("%s.pub", name))
}

// GetEtcdAltNames builds an AltNames object for generating the etcd server certificate.
func GetEtcdAltNames(cfg *apis.EtcdAdmConfig) (*certutil.AltNames, error) {
	// create AltNames with defaults DNSNames/IPs
	altNames := &certutil.AltNames{}

	appendSANsToAltNames(altNames, cfg.ServerCertSANs, constants.EtcdServerCertName)

	return altNames, nil
}

// GetEtcdPeerAltNames builds an AltNames object for generating the etcd peer certificate.
func GetEtcdPeerAltNames(cfg *apis.EtcdAdmConfig) (*certutil.AltNames, error) {
	// create AltNames with defaults DNSNames/IPs
	altNames := &certutil.AltNames{}

	appendSANsToAltNames(altNames, cfg.PeerCertSANs, constants.EtcdPeerCertName)

	return altNames, nil
}

// appendSANsToAltNames parses SANs from as list of strings and adds them to altNames for use on a specific cert
// altNames is passed in with a pointer, and the struct is modified
// valid IP address strings are parsed and added to altNames.IPs as net.IP's
// RFC-1123 compliant DNS strings are added to altNames.DNSNames as strings
// certNames is used to print user facing warnings and should be the name of the cert the altNames will be used for
func appendSANsToAltNames(altNames *certutil.AltNames, SANs []string, certName string) {
	for _, altname := range SANs {
		if ip := net.ParseIP(altname); ip != nil {
			altNames.IPs = append(altNames.IPs, ip)
		} else if len(validation.IsDNS1123Subdomain(altname)) == 0 {
			altNames.DNSNames = append(altNames.DNSNames, altname)
		} else {
			log.Printf(
				"[certificates] WARNING: '%s' was not added to the '%s' SAN, because it is not a valid IP or RFC-1123 compliant DNS entry\n",
				altname,
				certName,
			)
		}
	}
}
