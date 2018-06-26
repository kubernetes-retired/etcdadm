package cmd

import (
	"fmt"
	"log"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/platform9/etcdadm/apis"
	"github.com/platform9/etcdadm/binary"
	"github.com/platform9/etcdadm/certs"
	"github.com/platform9/etcdadm/constants"
	"github.com/platform9/etcdadm/service"
	"github.com/platform9/etcdadm/util"

	"github.com/spf13/cobra"
)

var joinCmd = &cobra.Command{
	Use:   "join",
	Short: "Join an existing etcd cluster",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(etcdAdmConfig.InitialClusterToken) == 0 {
			return fmt.Errorf("must provide cluster token")
		}
		if len(args) < 1 {
			return cobra.MinimumNArgs(1)(cmd, args)
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		endpoint := args[0]
		if _, err := url.Parse(endpoint); err != nil {
			log.Fatalf("Error: endpoint %q must be a valid URL: %s", endpoint, err)
		}

		var err error

		err = apis.SetJoinDynamicDefaults(&etcdAdmConfig)
		if err != nil {
			log.Fatalf("[defaults] Error: %s", err)
		}
		err = binary.EnsureInstalled(etcdAdmConfig.ReleaseURL, etcdAdmConfig.Version, etcdAdmConfig.InstallDir)
		if err != nil {
			log.Fatalf("[install] Error: %s", err)
		}
		err = certs.CreatePKIAssets(&etcdAdmConfig)
		if err != nil {
			log.Fatalf("[certificates] Error: %s", err)
		}

		mresp, err := util.AddSelfToEtcdCluster(endpoint, &etcdAdmConfig)
		// resp, err := cli.MemberList(context.Background())
		resp, err := util.MemberList(endpoint, &etcdAdmConfig)

		if err != nil {
			log.Fatalf("[cluster] Error: failed to list cluster members: %s", err)
		}
		conf := []string{}
		for _, memb := range resp.Members {
			for _, u := range memb.PeerURLs {
				n := memb.Name
				if memb.ID == mresp.Member.ID {
					n = etcdAdmConfig.Name
				}
				conf = append(conf, fmt.Sprintf("%s=%s", n, u))
			}
		}
		etcdAdmConfig.InitialCluster = strings.Join(conf, ",")
		// End

		err = service.WriteEnvironmentFile(&etcdAdmConfig)
		if err != nil {
			log.Fatalf("[configure] Error: %s", err)
		}
		err = service.WriteUnitFile(&etcdAdmConfig)
		if err != nil {
			log.Fatalf("[configure] Error: %s", err)
		}
		unit := filepath.Base(constants.UnitFile)
		err = service.EnableAndStartService(unit)
		if err != nil {
			log.Fatalf("[start] Error: %s", err)
		}
		err = service.WriteEtcdctlEnvFile(&etcdAdmConfig)
		if err != nil {
			log.Printf("[configure] Warning: %s", err)
		}

		// Verify cluster?
	},
}

func init() {
	rootCmd.AddCommand(joinCmd)
	joinCmd.PersistentFlags().StringVar(&etcdAdmConfig.Name, "name", "", "etcd member name")
	joinCmd.PersistentFlags().StringVar(&etcdAdmConfig.InitialClusterToken, "token", "", "initial cluster token")
	joinCmd.PersistentFlags().StringVar(&etcdAdmConfig.Version, "version", constants.DefaultVersion, "etcd version")
	joinCmd.PersistentFlags().StringVar(&etcdAdmConfig.ReleaseURL, "release-url", constants.DefaultReleaseURL, "URL used to download etcd")
	joinCmd.PersistentFlags().StringVar(&etcdAdmConfig.CertificatesDir, "certs-dir", constants.DefaultCertificateDir, "certificates directory")
	joinCmd.PersistentFlags().StringVar(&etcdAdmConfig.InstallBaseDir, "install-base-dir", constants.DefaultInstallBaseDir, "install base directory")
}
