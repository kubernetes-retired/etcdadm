/*
Copyright 2021 The Kubernetes Authors.

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
)

type CA struct {
	primaryCertificate *x509.Certificate
	privateKey         *rsa.PrivateKey
	certificates       []*x509.Certificate
}

func (c *CA) CertPool() *x509.CertPool {
	pool := x509.NewCertPool()
	for _, cert := range c.certificates {
		pool.AddCert(cert)
	}
	return pool
}
