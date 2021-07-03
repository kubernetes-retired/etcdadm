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
	"sync"

	certutil "k8s.io/client-go/util/cert"
)

type Store interface {
	Keypair(name string) MutableKeypair
}

// Keypairs manages a set of keypairs, providing utilities for fetching / creating them
type Keypairs struct {
	Store Store

	mutex sync.Mutex
	ca    *CA
}

// SetCA allows the CA to be set (if it has not yet been generated)
func (k *Keypairs) SetCA(ca *CA) {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	if k.ca != nil {
		panic("SetCA called when CA already set")
	}
	k.ca = ca
}

func (k *Keypairs) EnsureKeypair(name string, config certutil.Config, signer *CA) (*Keypair, error) {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	slot := k.Store.Keypair(name)
	keypair, err := EnsureKeypair(slot, config, signer)

	return keypair, err
}

func (k *Keypairs) CA() (*CA, error) {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	if k.ca == nil {
		caConfig := certutil.Config{CommonName: "ca"}
		slot := k.Store.Keypair("ca")
		keypair, err := EnsureKeypair(slot, caConfig, nil)
		if err != nil {
			return nil, err
		}
		k.ca = &CA{keypair: keypair}
	}
	return k.ca, nil
}
