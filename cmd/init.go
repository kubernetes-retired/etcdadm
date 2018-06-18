package cmd

import (
	"log"
	"path/filepath"

	"github.com/platform9/etcdadm/apis"
	"github.com/platform9/etcdadm/binary"
	"github.com/platform9/etcdadm/certs"
	"github.com/platform9/etcdadm/constants"

	"github.com/platform9/etcdadm/service"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new etcd cluster",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		etcdAdmConfig.Name = args[0]
		apis.SetInitDynamicDefaults(&etcdAdmConfig)

		var err error
		err = binary.EnsureInstalled(etcdAdmConfig.ReleaseURL, etcdAdmConfig.Version, etcdAdmConfig.InstallDir)
		if err != nil {
			log.Fatalf("[install] Error: %s", err)
		}
		err = certs.CreatePKIAssets(&etcdAdmConfig)
		if err != nil {
			log.Fatalf("[certificates] Error: %s", err)
		}
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
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.InitialClusterToken, "token", "", "initial cluster token")
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.Version, "version", constants.DefaultVersion, "etcd version")
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.ReleaseURL, "release-url", constants.DefaultReleaseURL, "URL used to download etcd")
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.CertificatesDir, "certs-dir", constants.DefaultCertificateDir, "certificates directory")
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.InstallBaseDir, "install-base-dir", constants.DefaultInstallBaseDir, "install base directory")
}
