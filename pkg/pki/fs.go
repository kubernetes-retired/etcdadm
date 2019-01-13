package pki

import (
	"bytes"
	"crypto/rsa"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	certutil "k8s.io/client-go/util/cert"
	"k8s.io/klog"
)

func LoadCAFromDisk(dir string) (*Keypair, error) {
	klog.Infof("Loading certificate authority from %v", dir)
	certPath := filepath.Join(dir, "ca.crt")
	certMaterial, err := ioutil.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read cert %v: %v", certPath, err)
	}
	keyPath := filepath.Join(dir, "ca.key")
	keyMaterial, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read key %v: %v", keyPath, err)
	}
	cert, err := ParseOneCertificate(certMaterial)
	if err != nil {
		return nil, fmt.Errorf("error parsing cert %q: %v", certPath, err)
	}

	key, err := certutil.ParsePrivateKeyPEM(keyMaterial)
	if err != nil {
		return nil, fmt.Errorf("unable to parse private key %q: %v", keyPath, err)
	}

	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("unexpected private key type in %q: %T", keyPath, key)
	}

	ca := &Keypair{
		Certificate:    cert,
		CertificatePEM: certMaterial,
		PrivateKey:     rsaKey,
		PrivateKeyPEM:  keyMaterial,
	}
	return ca, nil
}

type MutableKeypairFromFile struct {
	PrivateKeyPath  string
	CertificatePath string
}

var _ MutableKeypair = &MutableKeypairFromFile{}

func (s *MutableKeypairFromFile) MutateKeypair(mutator func(keypair *Keypair) error) (*Keypair, error) {
	keypair := &Keypair{}

	privateKeyBytes, err := ioutil.ReadFile(s.PrivateKeyPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("unable to read key %v: %v", s.PrivateKeyPath, err)
		}
		privateKeyBytes = nil
	}

	if privateKeyBytes != nil {
		key, err := certutil.ParsePrivateKeyPEM(privateKeyBytes)
		if err != nil {
			return nil, fmt.Errorf("unable to parse private key %q: %v", s.PrivateKeyPath, err)
		}

		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("unexpected private key type in %q: %T", s.PrivateKeyPath, key)
		}
		keypair.PrivateKey = rsaKey
		keypair.PrivateKeyPEM = privateKeyBytes
	}

	certBytes, err := ioutil.ReadFile(s.CertificatePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("unable to read certificate %v: %v", s.CertificatePath, err)
		}
		certBytes = nil
	}

	if certBytes != nil {
		cert, err := ParseOneCertificate(certBytes)
		if err != nil {
			return nil, fmt.Errorf("error parsing certificate data in %q: %v", s.CertificatePath, err)
		}

		keypair.Certificate = cert
		keypair.CertificatePEM = certBytes
	}

	if err := mutator(keypair); err != nil {
		return nil, err
	}

	if !bytes.Equal(privateKeyBytes, keypair.PrivateKeyPEM) {
		if err := os.MkdirAll(filepath.Dir(s.PrivateKeyPath), 0755); err != nil {
			return nil, fmt.Errorf("error creating directories for private key file %q: %v", s.PrivateKeyPath, err)
		}

		if err := ioutil.WriteFile(s.PrivateKeyPath, keypair.PrivateKeyPEM, 0600); err != nil {
			return nil, fmt.Errorf("error writing private key file %q: %v", s.PrivateKeyPath, err)
		}
	}

	if !bytes.Equal(certBytes, keypair.CertificatePEM) {
		if err := os.MkdirAll(filepath.Dir(s.CertificatePath), 0755); err != nil {
			return nil, fmt.Errorf("error creating directories for certificate file %q: %v", s.CertificatePath, err)
		}

		if err := ioutil.WriteFile(s.CertificatePath, keypair.CertificatePEM, 0644); err != nil {
			return nil, fmt.Errorf("error writing certificate key file %q: %v", s.CertificatePath, err)
		}
	}

	return keypair, nil
}
