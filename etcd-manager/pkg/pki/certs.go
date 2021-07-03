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
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	certutil "k8s.io/client-go/util/cert"
	"k8s.io/klog/v2"
)

// CertDuration controls how long we issue certificates for.  We set
// it to a longer time period, primarily because we don't have a nice
// means of rotation.  This was historically one year, but we now set
// it to two years, with kubernetes LTS proposing one year's support.
var CertDuration = 2 * 365 * 24 * time.Hour

// CertMinTimeLeft is the minimum amount of validity required on
// a certificate to reuse it.  Because we set this (much) higher than
// CertDuration, we will now always reissue certificates.
var CertMinTimeLeft = 20 * 365 * 24 * time.Hour

// ParseHumanDuration parses a go-style duration string, but
// recognizes additional suffixes: d means "day" and is interpreted as
// 24 hours; y means "year" and is interpreted as 365 days.
func ParseHumanDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "y") {
		s = strings.TrimSuffix(s, "y")
		n, err := strconv.Atoi(s)
		if err != nil {
			return time.Duration(0), err
		}
		return time.Duration(n) * 365 * 24 * time.Hour, nil
	}

	if strings.HasSuffix(s, "d") {
		s = strings.TrimSuffix(s, "d")
		n, err := strconv.Atoi(s)
		if err != nil {
			return time.Duration(0), err
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}

	return time.ParseDuration(s)

}

func init() {
	if s := os.Getenv("ETCD_MANAGER_CERT_DURATION"); s != "" {
		v, err := ParseHumanDuration(s)
		if err != nil {
			klog.Fatalf("failed to parse ETCD_MANAGER_CERT_DURATION=%q", s)
		}
		CertDuration = v
	}

	if s := os.Getenv("ETCD_MANAGER_CERT_MIN_TIME_LEFT"); s != "" {
		v, err := ParseHumanDuration(s)
		if err != nil {
			klog.Fatalf("failed to parse ETCD_MANAGER_CERT_MIN_TIME_LEFT=%q", s)
		}
		CertMinTimeLeft = v
	}
}

type Keypair struct {
	Certificate *x509.Certificate
	PrivateKey  *rsa.PrivateKey
}

type MutableKeypair interface {
	MutateKeypair(mutator func(keypair *Keypair) error) (*Keypair, error)
}

func newCAKeypair(store MutableKeypair, config certutil.Config) (*Keypair, error) {
	p := config.CommonName

	mutator := func(keypair *Keypair) error {
		privateKey, err := newPrivateKey()
		if err != nil {
			return fmt.Errorf("unable to create private key %q: %v", p, err)
		}
		keypair.PrivateKey = privateKey

		klog.Infof("generating certificate for %q", p)
		cert, err := certutil.NewSelfSignedCACert(config, keypair.PrivateKey)
		if err != nil {
			return fmt.Errorf("error signing certificate for %q: %v", p, err)
		}
		keypair.Certificate = cert

		return nil
	}

	keypair, err := store.MutateKeypair(mutator)
	if err != nil {
		return nil, err
	}

	return keypair, nil
}

func ensureKeypair(store MutableKeypair, config certutil.Config, signer *CA) (*Keypair, error) {
	p := config.CommonName

	mutator := func(keypair *Keypair) error {
		if keypair.PrivateKey == nil {
			privateKey, err := newPrivateKey()
			if err != nil {
				return fmt.Errorf("unable to create private key %q: %v", p, err)
			}
			keypair.PrivateKey = privateKey
		}

		if keypair.Certificate != nil {
			cert := keypair.Certificate

			match := true

			if match && time.Until(cert.NotAfter) <= CertMinTimeLeft {
				klog.Infof("existing certificate not valid after %s; will regenerate", cert.NotAfter.Format(time.RFC3339))
				match = false
			}

			if match && cert.Subject.CommonName != config.CommonName {
				klog.Infof("certificate CommonName mismatch on %q; will regenerate", p)
				match = false
			}

			// TODO: Organization
			// TODO: Usages

			if match {
				var expectedNames []string
				var actualNames []string

				for _, s := range config.AltNames.DNSNames {
					expectedNames = append(expectedNames, s)
				}

				for _, s := range cert.DNSNames {
					actualNames = append(actualNames, s)
				}

				sort.Strings(expectedNames)
				sort.Strings(actualNames)

				if !reflect.DeepEqual(expectedNames, actualNames) {
					klog.Infof("certificate DNS names mismatch on %q; will regenerate", p)
					match = false
				}
			}

			if match {
				var expectedIPs []string
				var actualIPs []string

				for _, s := range config.AltNames.IPs {
					expectedIPs = append(expectedIPs, s.String())
				}

				for _, s := range cert.IPAddresses {
					actualIPs = append(actualIPs, s.String())
				}

				sort.Strings(expectedIPs)
				sort.Strings(actualIPs)

				if !reflect.DeepEqual(expectedIPs, actualIPs) {
					klog.Infof("certificate IPs mismatch on %q; will regenerate", p)
					match = false
				}
			}

			if !match {
				keypair.Certificate = nil
			}
		}

		if keypair.Certificate == nil {
			klog.Infof("generating certificate for %q", p)
			cert, err := newSignedCert(&config, keypair.PrivateKey, signer, CertDuration)
			if err != nil {
				return fmt.Errorf("error signing certificate for %q: %v", p, err)
			}

			keypair.Certificate = cert
		}

		return nil
	}

	keypair, err := store.MutateKeypair(mutator)
	if err != nil {
		return nil, err
	}

	return keypair, nil
}
