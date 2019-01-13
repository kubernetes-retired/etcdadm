package pki

import (
	"path/filepath"
	"sync"

	certutil "k8s.io/client-go/util/cert"
)

type Store interface {
	Keypair(name string) MutableKeypair
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
	p := name
	return &MutableKeypairFromFile{
		PrivateKeyPath:  filepath.Join(s.basedir, p+".key"),
		CertificatePath: filepath.Join(s.basedir, p+".crt"),
	}
}

// Keypairs manages a set of keypairs, providing utilities for fetching / creating them
type Keypairs struct {
	Store Store

	mutex sync.Mutex
	ca    *Keypair
}

// SetCA allows the CA to be set (if it has not yet been generated)
func (k *Keypairs) SetCA(ca *Keypair) {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	if k.ca != nil {
		panic("SetCA called when CA already set")
	}
	k.ca = ca
}

func (k *Keypairs) EnsureKeypair(name string, config certutil.Config, signer *Keypair) (*Keypair, error) {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	slot := k.Store.Keypair(name)
	keypair, err := EnsureKeypair(slot, config, signer)

	return keypair, err
}

func (k *Keypairs) CA() (*Keypair, error) {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	if k.ca == nil {
		caConfig := certutil.Config{CommonName: "ca"}
		slot := k.Store.Keypair("ca")
		keypair, err := EnsureKeypair(slot, caConfig, nil)
		if err != nil {
			return nil, err
		}
		k.ca = keypair
	}
	return k.ca, nil
}
