package pki

import (
	"crypto/x509"
	"fmt"

	certutil "k8s.io/client-go/util/cert"
)

func ParseOneCertificate(b []byte) (*x509.Certificate, error) {
	certs, err := certutil.ParseCertsPEM(b)
	if err != nil {
		return nil, fmt.Errorf("error parsing certificate data: %v", err)
	}

	if len(certs) > 1 {
		// TODO: Handle this?
		return nil, fmt.Errorf("found multiple certificates")
	}

	if len(certs) == 0 {
		return nil, fmt.Errorf("did not find any certificates")
	}

	return certs[0], nil
}
