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
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"
	"testing"

	certutil "k8s.io/client-go/util/cert"
)

func TestFSStore_LoadCA(t *testing.T) {
	const basicCert = "-----BEGIN CERTIFICATE-----\nMIIBaDCCARKgAwIBAgIMFoq6Pex4lTCM8fOIMA0GCSqGSIb3DQEBCwUAMBUxEzAR\nBgNVBAMTCmt1YmVybmV0ZXMwHhcNMjEwNjE5MjI0MzEwWhcNMzEwNjE5MjI0MzEw\nWjAVMRMwEQYDVQQDEwprdWJlcm5ldGVzMFwwDQYJKoZIhvcNAQEBBQADSwAwSAJB\nANiW3hfHTcKnxCig+uWhpVbOfH1pANKmXVSysPKgE80QSU4tZ6m49pAEeIMsvwvD\nMaLsb2v6JvXe0qvCmueU+/sCAwEAAaNCMEAwDgYDVR0PAQH/BAQDAgEGMA8GA1Ud\nEwEB/wQFMAMBAf8wHQYDVR0OBBYEFCOW3hR7ngBsk9aUOlEznWzH494EMA0GCSqG\nSIb3DQEBCwUAA0EAVnZzkiku07kQFGAEXzWI6aZnAbzSoClYskEzCBMrOmdadjVp\nVWcz76FwFlyd5jhzOJ49eMcVusSotKv2ZGimcA==\n-----END CERTIFICATE-----"
	const basicKey = "-----BEGIN RSA PRIVATE KEY-----\nMIIBPQIBAAJBANiW3hfHTcKnxCig+uWhpVbOfH1pANKmXVSysPKgE80QSU4tZ6m4\n9pAEeIMsvwvDMaLsb2v6JvXe0qvCmueU+/sCAwEAAQJBAKt/gmpHqP3qA3u8RA5R\n2W6L360Z2Mnza1FmkI/9StCCkJGjuE5yDhxU4JcVnFyX/nMxm2ockEEQDqRSu7Oo\nxTECIQD2QsUsgFL4FnXWzTclySJ6ajE4Cte3gSDOIvyMNMireQIhAOEnsV8UaSI+\nZyL7NMLzMPLCgtsrPnlamr8gdrEHf9ITAiEAxCCLbpTI/4LL2QZZrINTLVGT34Fr\nKl/yI5pjrrp/M2kCIQDfOktQyRuzJ8t5kzWsUxCkntS+FxHJn1rtQ3Jp8dV4oQIh\nAOyiVWDyLZJvg7Y24Ycmp86BZjM9Wk/BfWpBXKnl9iDY\n-----END RSA PRIVATE KEY-----"
	tests := []struct {
		name           string
		cert           string
		key            string
		expectedPool   string
		expectedBundle string
		expectedErr    string
	}{
		{
			name:           "basic",
			cert:           basicCert,
			key:            basicKey,
			expectedPool:   "CN=kubernetes",
			expectedBundle: basicCert + "\n",
		},
		{
			name:        "badcert",
			cert:        "not a cert",
			key:         basicKey,
			expectedErr: "error parsing certificate data in ",
		},
		{
			name:        "badkey",
			cert:        basicCert,
			key:         "not a key",
			expectedErr: "unable to parse private key ",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tempDir, err := ioutil.TempDir("", "test")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer func() {
				if os.Getenv("KEEP_TEMP_DIR") != "" {
					t.Logf("NOT removing temp directory, because KEEP_TEMP_DIR is set: %s", tempDir)
				} else {
					err := os.RemoveAll(tempDir)
					if err != nil {
						t.Fatalf("failed to remove temp dir %q: %v", tempDir, err)
					}
				}
			}()

			if tc.cert != "" {
				_ = os.WriteFile(path.Join(tempDir, "test-ca.crt"), []byte(tc.cert), 0400)
			}
			if tc.key != "" {
				_ = os.WriteFile(path.Join(tempDir, "test-ca.key"), []byte(tc.key), 0400)
			}

			store := NewFSStore(tempDir)
			actual, err := store.LoadCA("test-ca")
			if err != nil && tc.expectedErr == "" {
				t.Fatalf("unexpected error %v", err)
			}
			if err != nil && !strings.Contains(err.Error(), tc.expectedErr) {
				t.Fatalf("error = %v, expected %s", err, tc.expectedErr)
			}
			if err != nil {
				return
			}
			if tc.expectedErr != "" {
				t.Fatalf("did not get expected error %s", tc.expectedErr)
			}

			var subjects []string
			for _, subject := range actual.CertPool().Subjects() {
				var name pkix.RDNSequence
				rest, err := asn1.Unmarshal(subject, &name)
				if err != nil {
					t.Fatalf("subject unmarshal error %v", err)
				}
				if len(rest) > 0 {
					t.Fatalf("extra data after unmarshalling subject")
				}
				subjects = append(subjects, name.String())
			}
			sort.Strings(subjects)
			if strings.Join(subjects, "\n") != tc.expectedPool {
				t.Fatalf("unexpected pool subjects %s, expected %s", strings.Join(subjects, "\n"), tc.expectedPool)
			}

			err = store.WriteCABundle(actual)
			if err != nil {
				t.Errorf("writing CA bundle: %v", err)
			} else {
				bytes, err := os.ReadFile(path.Join(tempDir, "ca.crt"))
				if err != nil {
					t.Errorf("writing CA bundle: %v", err)
				} else if string(bytes) != tc.expectedBundle {
					t.Errorf("unexpected bundle. actual:\n%s\nexpected:\n%s\n", string(bytes), tc.expectedBundle)
				}
			}

			keypairs := NewKeypairs(NewInMemoryStore(), actual)
			keypair, err := keypairs.EnsureKeypair("test-cert", certutil.Config{
				CommonName: "test-cert",
				Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			})
			if err != nil {
				t.Errorf("ensuring keypair: %s", err)
			} else if keypair.Certificate.Subject.CommonName != "test-cert" {
				t.Errorf("unexpected subject")
			}
		})
	}
}
