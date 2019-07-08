/*
Copyright 2018 The Kubernetes Authors.

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
	"context"
	"os"

	log "sigs.k8s.io/etcdadm/pkg/logrus"

	"github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/spf13/cobra"
	"sigs.k8s.io/etcdadm/apis"
	"sigs.k8s.io/etcdadm/binary"
	"sigs.k8s.io/etcdadm/constants"
	"sigs.k8s.io/etcdadm/etcd"
	"sigs.k8s.io/etcdadm/service"
)

var skipRemoveMember bool

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Remove this etcd member from the cluster and uninstall etcd",
	Run: func(cmd *cobra.Command, args []string) {
		// Load constants & defaults
		apis.SetDefaults(&etcdAdmConfig)
		err := apis.SetResetDynamicDefaults(&etcdAdmConfig)
		if err != nil {
			log.Fatalf("[defaults] Error: %s", err)
		}

		active, err := service.Active(constants.UnitFileBaseName)
		if err != nil {
			log.Fatalf("[reset] Error checking if etcd service is active: %s", err)
		}
		if !active {
			log.Println("[reset] etcd service is not running")
			log.Println("[membership] Assuming that the member was removed")
		} else {
			log.Println("[reset] etcd service is running")
			// Remove self as member from etcd cluster
			if !skipRemoveMember {
				var localMember *etcdserverpb.Member
				log.Println("[membership] Checking if this member was removed")
				client, err := etcd.ClientForEndpoint(etcdAdmConfig.LoopbackClientURL.String(), &etcdAdmConfig)
				if err != nil {
					log.Fatalf("[membership] Error checking membership: %v", err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultEtcdRequestTimeout)
				mresp, err := client.MemberList(ctx)
				cancel()
				if err != nil {
					log.Fatalf("[membership] Error listing members: %v", err)
				}
				localMember, ok := etcd.MemberForPeerURLs(mresp.Members, etcdAdmConfig.InitialAdvertisePeerURLs.StringSlice())
				if ok {
					log.Println("[membership] Member was not removed")
					if len(mresp.Members) > 1 {
						log.Println("[membership] Removing member")
						ctx, cancel = context.WithTimeout(context.Background(), constants.DefaultEtcdRequestTimeout)
						_, err = client.MemberRemove(ctx, localMember.ID)
						cancel()
						if err != nil {
							log.Fatalf("[membership] Error removing member: %v", err)
						}
					} else {
						log.Println("[membership] Not removing member because it is the last in the cluster")
					}
				} else {
					log.Println("[membership] Member was removed")
				}
			}
			// Disable etcd service
			if err := service.Stop(constants.UnitFileBaseName); err != nil {
				log.Fatalf("[reset] Error stopping existing etcd service: %s", err)
			}
		}
		enabled, err := service.Enabled(constants.UnitFileBaseName)
		if err != nil {
			log.Fatalf("[reset] Error checking if etcd service is enabled: %s", err)
		}
		if enabled {
			if err := service.Disable(constants.UnitFileBaseName); err != nil {
				log.Fatalf("[reset] Error disabling existing etcd service: %s", err)
			}
		}
		// Remove etcd datastore
		if err = os.RemoveAll(etcdAdmConfig.DataDir); err != nil {
			log.Print(err)
		}
		// Remove configuration files
		if err = os.RemoveAll(etcdAdmConfig.CertificatesDir); err != nil {
			log.Print(err)
		}
		if err = os.Remove(etcdAdmConfig.UnitFile); err != nil {
			log.Print(err)
		}
		if err = os.Remove(etcdAdmConfig.EnvironmentFile); err != nil {
			log.Print(err)
		}
		if err = os.Remove(etcdAdmConfig.EtcdctlEnvFile); err != nil {
			log.Print(err)
		}
		// Remove binaries
		if err := binary.Uninstall(etcdAdmConfig.Version, etcdAdmConfig.InstallDir); err != nil {
			log.Printf("[binaries] Unable to uninstall binaries: %v", err)
		}
		if err = os.Remove(etcdAdmConfig.EtcdctlShellWrapper); err != nil {
			log.Print(err)
		}
		log.Printf("[cluster] etcd reset complete")
	},
}

func init() {
	rootCmd.AddCommand(resetCmd)
	resetCmd.Flags().BoolVar(&skipRemoveMember, "skip-remove-member", constants.DefaultSkipRemoveMember, "Use skip-remove-member flag to skip the process of removing member from etcd cluster but clean everything else.")
	resetCmd.PersistentFlags().StringVar(&etcdAdmConfig.InstallDir, "install-dir", constants.DefaultInstallDir, "install directory")
	resetCmd.PersistentFlags().StringVar(&etcdAdmConfig.CertificatesDir, "certs-dir", constants.DefaultCertificateDir, "certificates directory")
}
