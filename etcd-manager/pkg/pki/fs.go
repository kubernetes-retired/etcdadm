/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pki

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"k8s.io/client-go/util/keyutil"
)

type MutableKeypairFromFile struct {
	PrivateKeyPath  string
	CertificatePath string
}

var _ MutableKeypair = &MutableKeypairFromFile{}

func (s *MutableKeypairFromFile) MutateKeypair(mutator func(keypair *Keypair) error) (*Keypair, error) {
	keypair := &Keypair{}
	if err := loadPrivateKey(s.PrivateKeyPath, keypair); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		// We tolerate a missing key when generating the keypair
	}
	if err := loadCertificate(s.CertificatePath, keypair); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		// We tolerate a missing cert when generating the keypair
	}

	original := *keypair

	if err := mutator(keypair); err != nil {
		return nil, err
	}

	if original.PrivateKey == nil || !original.PrivateKey.Equal(keypair.PrivateKey) {
		if err := writePrivateKey(s.PrivateKeyPath, keypair.PrivateKey); err != nil {
			return nil, err
		}
	}

	if original.Certificate == nil || !original.Certificate.Equal(keypair.Certificate) {
		if err := writeCertificate(s.CertificatePath, keypair.Certificate); err != nil {
			return nil, err
		}
	}

	return keypair, nil
}

type FSStore struct {
	basedir string
}

var _ Store = &FSStore{}

func NewFSStore(basedir string) *FSStore {
	return &FSStore{
		basedir: basedir,
	}
}

func (s *FSStore) Keypair(name string) MutableKeypair {
	return &MutableKeypairFromFile{
		PrivateKeyPath:  filepath.Join(s.basedir, name+".key"),
		CertificatePath: filepath.Join(s.basedir, name+".crt"),
	}
}

func writePrivateKey(path string, privateKey *rsa.PrivateKey) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating directories for private key file %q: %v", path, err)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("opening private key file %q: %v", path, err)
	}

	err = pem.Encode(f, &pem.Block{Type: RSAPrivateKeyBlockType, Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("writing private key file %q: %v", path, err)
	}

	err = f.Close()
	if err != nil {
		return fmt.Errorf("closing private key file %q: %v", path, err)
	}

	return nil
}

func (s *FSStore) WriteCertificate(name string, keypair *Keypair) error {
	p := filepath.Join(s.basedir, name+".crt")

	return writeCertificate(p, keypair.Certificate)
}

func writeCertificate(path string, certificate *x509.Certificate) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating directories for certificate file %q: %v", path, err)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("opening certificate file %q: %v", path, err)
	}

	err = pem.Encode(f, &pem.Block{Type: CertificateBlockType, Bytes: certificate.Raw})
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("writing certificate file %q: %v", path, err)
	}

	err = f.Close()
	if err != nil {
		return fmt.Errorf("closing certificate file %q: %v", path, err)
	}

	return nil
}

func (s *FSStore) LoadKeypair(name string) (*Keypair, error) {
	keypair := &Keypair{}
	if err := loadPrivateKey(filepath.Join(s.basedir, name+".key"), keypair); err != nil {
		return nil, err
	}
	if err := loadCertificate(filepath.Join(s.basedir, name+".crt"), keypair); err != nil {
		return nil, err
	}
	return keypair, nil
}

func loadPrivateKey(privateKeyPath string, keypair *Keypair) error {
	privateKeyBytes, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("unable to read key %v: %v", privateKeyPath, err)
		} else {
			return err
		}
	}

	if privateKeyBytes != nil {
		key, err := keyutil.ParsePrivateKeyPEM(privateKeyBytes)
		if err != nil {
			return fmt.Errorf("unable to parse private key %q: %v", privateKeyPath, err)
		}

		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return fmt.Errorf("unexpected private key type in %q: %T", privateKeyPath, key)
		}
		keypair.PrivateKey = rsaKey
	}

	return nil
}

func loadCertificate(certificatePath string, keypair *Keypair) error {
	certBytes, err := ioutil.ReadFile(certificatePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("unable to read certificate %v: %v", certificatePath, err)
		}
		certBytes = nil
	}

	if certBytes != nil {
		cert, err := ParseOneCertificate(certBytes)
		if err != nil {
			return fmt.Errorf("error parsing certificate data in %q: %v", certificatePath, err)
		}

		keypair.Certificate = cert
	}

	return nil
}
