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

package tlsconfig

import (
	"crypto/tls"
	"crypto/x509"

	certutil "k8s.io/client-go/util/cert"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/pki"
)

func GRPCClientConfig(keypairs *pki.Keypairs, myPeerID string) (*tls.Config, error) {
	ca, err := keypairs.CA()
	if err != nil {
		return nil, err
	}

	caPool := x509.NewCertPool()
	caPool.AddCert(ca.Certificate)

	keypair, err := keypairs.EnsureKeypair("etcd-manager-client-"+myPeerID, certutil.Config{
		CommonName: "etcd-manager-client-" + myPeerID,
		Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}, ca)
	if err != nil {
		return nil, err
	}

	c := &tls.Config{
		RootCAs: caPool,
	}
	c.Certificates = append(c.Certificates, tls.Certificate{
		Certificate: [][]byte{keypair.Certificate.Raw},
		PrivateKey:  keypair.PrivateKey,
		Leaf:        keypair.Certificate,
	})

	return c, nil
}

func GRPCServerConfig(keypairs *pki.Keypairs, myPeerID string) (*tls.Config, error) {
	ca, err := keypairs.CA()
	if err != nil {
		return nil, err
	}

	caPool := x509.NewCertPool()
	caPool.AddCert(ca.Certificate)

	config := certutil.Config{
		CommonName: "etcd-manager-server-" + myPeerID,
		Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	keypair, err := keypairs.EnsureKeypair("etcd-manager-server-"+myPeerID, config, ca)
	if err != nil {
		return nil, err
	}

	c := &tls.Config{
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  caPool,
		ServerName: "etcd-manager-server-" + myPeerID,
	}
	c.Certificates = append(c.Certificates, tls.Certificate{
		Certificate: [][]byte{keypair.Certificate.Raw},
		PrivateKey:  keypair.PrivateKey,
		Leaf:        keypair.Certificate,
	})

	return c, nil
}
