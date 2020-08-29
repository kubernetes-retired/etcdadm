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
	"bytes"
	"crypto/rsa"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"k8s.io/client-go/util/keyutil"
	"k8s.io/klog"
)

func LoadCAFromDisk(dir string) (*Keypair, error) {
	klog.Infof("Loading certificate authority from %v", dir)

	keypair := &Keypair{}

	if err := loadPrivateKey(filepath.Join(dir, "ca.key"), keypair); err != nil {
		return nil, err
	}
	if err := loadCertificate(filepath.Join(dir, "ca.crt"), keypair); err != nil {
		return nil, err
	}

	return keypair, nil
}

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

	if !bytes.Equal(original.PrivateKeyPEM, keypair.PrivateKeyPEM) {
		if err := os.MkdirAll(filepath.Dir(s.PrivateKeyPath), 0755); err != nil {
			return nil, fmt.Errorf("error creating directories for private key file %q: %v", s.PrivateKeyPath, err)
		}

		if err := ioutil.WriteFile(s.PrivateKeyPath, keypair.PrivateKeyPEM, 0600); err != nil {
			return nil, fmt.Errorf("error writing private key file %q: %v", s.PrivateKeyPath, err)
		}
	}

	if !bytes.Equal(original.CertificatePEM, keypair.CertificatePEM) {
		// TODO: Replace with simpler call to WriteCertificate?
		if err := os.MkdirAll(filepath.Dir(s.CertificatePath), 0755); err != nil {
			return nil, fmt.Errorf("error creating directories for certificate file %q: %v", s.CertificatePath, err)
		}

		if err := ioutil.WriteFile(s.CertificatePath, keypair.CertificatePEM, 0644); err != nil {
			return nil, fmt.Errorf("error writing certificate key file %q: %v", s.CertificatePath, err)
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

func (s *FSStore) WriteCertificate(name string, keypair *Keypair) error {
	p := filepath.Join(s.basedir, name+".crt")

	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return fmt.Errorf("error creating directories for certificate file %q: %v", p, err)
	}

	if err := ioutil.WriteFile(p, keypair.CertificatePEM, 0644); err != nil {
		return fmt.Errorf("error writing certificate key file %q: %v", p, err)
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
		keypair.PrivateKeyPEM = privateKeyBytes
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
		keypair.CertificatePEM = certBytes
	}

	return nil
}
