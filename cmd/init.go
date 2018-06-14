package cmd

import (
	"log"

	"github.com/platform9/etcdadm/constants"

	"github.com/platform9/etcdadm/apis"
	"github.com/platform9/etcdadm/binary"
	"github.com/platform9/etcdadm/service"
	"github.com/spf13/cobra"
)

var etcdAdmConfig apis.EtcdAdmConfig

// createCmd represents the create command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new etcd cluster",
	Run: func(cmd *cobra.Command, args []string) {
		err := binary.EnsureInstalled(etcdAdmConfig.ReleaseURL, etcdAdmConfig.Version, etcdAdmConfig.InstallDir)
		if err != nil {
			log.Fatalf("Error installing etcd: %s", err)
		}
		err = service.WriteUnitFile(&etcdAdmConfig)
		if err != nil {
			log.Fatalf("Error configuring etcd: %s", err)
		}
		err = service.WriteEnvironmentFile(&etcdAdmConfig)
		if err != nil {
			log.Fatalf("Error configuring etcd: %s", err)
		}
		err = service.EnableAndStartService()
		if err != nil {
			log.Fatalf("Error running etcd: %s", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.Version, "version", constants.DefaultVersion, "etcd version")
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.ReleaseURL, "release-url", constants.DefaultReleaseURL, "URL used to download etcd")
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.CertificatesDir, "certs-dir", constants.DefaultCertificateDir, "certificates directory")
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.InstallDir, "install-dir", constants.DefaultInstallDir(), "install directory")
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.Name, "name", "", "etcd member name")
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.InitialClusterToken, "initial-cluster-token", "", "initial cluster token")
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.InitialCluster, "initial-cluster", "", "initial cluster")
}
