package pki

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"reflect"
	"sort"

	certutil "k8s.io/client-go/util/cert"
	"k8s.io/klog"
)

type Keypair struct {
	Certificate    *x509.Certificate
	CertificatePEM []byte
	PrivateKey     *rsa.PrivateKey
	PrivateKeyPEM  []byte
}

type MutableKeypair interface {
	MutateKeypair(mutator func(keypair *Keypair) error) (*Keypair, error)
}

func EnsureKeypair(store MutableKeypair, config certutil.Config, signer *Keypair) (*Keypair, error) {
	// TODO: Use the kops Keyset type

	p := config.CommonName

	mutator := func(keypair *Keypair) error {
		if keypair.PrivateKey == nil {
			privateKey, err := NewPrivateKey()
			if err != nil {
				return fmt.Errorf("unable to create private key %q: %v", p, err)
			}
			b := EncodePrivateKeyPEM(privateKey)
			keypair.PrivateKey = privateKey
			keypair.PrivateKeyPEM = b
		}

		if keypair.Certificate != nil {
			cert := keypair.Certificate

			match := true

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
				keypair.CertificatePEM = nil
			}
		}

		if keypair.Certificate == nil {
			klog.Infof("generating certificate for %q", p)
			var cert *x509.Certificate
			var err error
			if signer != nil {
				cert, err = NewSignedCert(&config, keypair.PrivateKey, signer.Certificate, signer.PrivateKey)
			} else {
				cert, err = certutil.NewSelfSignedCACert(config, keypair.PrivateKey)
			}

			if err != nil {
				return fmt.Errorf("error signing certificate for %q: %v", p, err)
			}

			b := EncodeCertPEM(cert)
			keypair.Certificate = cert
			keypair.CertificatePEM = b
		}

		return nil
	}

	keypair, err := store.MutateKeypair(mutator)
	if err != nil {
		return nil, err
	}

	return keypair, nil
}
