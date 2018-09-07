package cmd

import (
	"context"
	"log"
	"os"

	"github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/platform9/etcdadm/apis"
	"github.com/platform9/etcdadm/binary"
	"github.com/platform9/etcdadm/constants"
	"github.com/platform9/etcdadm/etcd"
	"github.com/platform9/etcdadm/service"
	"github.com/spf13/cobra"
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
					log.Printf("[membership] Error checking membership: %v", err)
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
