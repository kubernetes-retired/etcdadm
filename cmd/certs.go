/*
Copyright 2019 The Kubernetes Authors.

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

package cmd

import (
	"github.com/spf13/cobra"
	"sigs.k8s.io/etcdadm/certs"
	"sigs.k8s.io/etcdadm/constants"
	log "sigs.k8s.io/etcdadm/pkg/logrus"
)

// newCmdCerts returns main command for certs phase
func newCmdCerts() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "certs",
		Short: "Commands related to handling etcdadm certificates",
	}

	cmd.AddCommand(newCmdCertsRenewal())

	return cmd
}

func newCmdCertsRenewal() *cobra.Command {
	var certificatesDir string
	var csrOnly bool
	var csrDir string
	cmd := &cobra.Command{
		Use:   "renew",
		Short: "Renew certificates for a etcd cluster",
		Run: func(cmd *cobra.Command, args []string) {
			for _, name := range certs.GetDefaultCertList() {
				log.Printf("[renew] Renew etcd %s certificate.", name)
				var renewed bool
				var err error
				if csrOnly {
					renewed, err = certs.RenewCSRUsingLocalCA(certificatesDir, name, csrDir)
				} else {
					renewed, err = certs.RenewUsingLocalCA(certificatesDir, name)
				}

				if err != nil {
					log.Fatalf("[renew] Error renew certificate %s: %v", name, err)
				}
				if !renewed {
					log.Fatalf("Certificates %s can't be renewed.\n", name)
				}
			}
			log.Println("The etcd certificates have been renewed successfully!")
			log.Warnln("If kube-apiserver which version less than v1.10.0 is running, restart it so that it uses the renewed etcd client certificate.")
		},
	}
	cmd.PersistentFlags().StringVar(&certificatesDir, "certs-dir", constants.DefaultCertificateDir, "certificates directory")
	cmd.PersistentFlags().BoolVar(&csrOnly, "csr-only", false, "create CSRs instead of generating certificates")
	cmd.PersistentFlags().StringVar(&csrDir, "csr-dir", "./", "directory to output CSRs to")

	return cmd
}

func init() {
	rootCmd.AddCommand(newCmdCerts())
}
