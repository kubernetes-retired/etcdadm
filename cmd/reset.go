package cmd

import (
	"log"
	"os"
	"path/filepath"

	"github.com/platform9/etcdadm/apis"
	"github.com/platform9/etcdadm/binary"
	"github.com/platform9/etcdadm/constants"
	"github.com/platform9/etcdadm/service"
	"github.com/platform9/etcdadm/util"
	"github.com/spf13/cobra"
)

var skipRemoveMember bool

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset a new etcd cluster",
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		// Load constants & defaults
		apis.SetDefaults(&etcdAdmConfig)
		err = apis.SetResetDynamicDefaults(&etcdAdmConfig)
		if err != nil {
			log.Fatalf("[defaults] Error: %s", err)
		}
		// Remove self as member from etcd cluster
		if !skipRemoveMember {
			err = util.RemoveSelfFromEtcdCluster(&etcdAdmConfig)
			if err != nil {
				log.Fatal(err)
			}
		}
		// Remove etcd datastore
		if err = os.RemoveAll(constants.DefaultDataDir); err != nil {
			log.Print(err)
		}
		// Disable and stop etcd service
		unit := filepath.Base(constants.UnitFile)
		service.DisableAndStopService(unit)
		// Remove configuration files
		if err = os.RemoveAll(constants.DefaultCertificateDir); err != nil {
			log.Print(err)
		}
		if err = os.Remove(constants.UnitFile); err != nil {
			log.Print(err)
		}
		if err = os.Remove(constants.EnvironmentFile); err != nil {
			log.Print(err)
		}
		if err = os.Remove(constants.EtcdctlEnvFile); err != nil {
			log.Print(err)
		}
		// Remove binaries
		if err = os.RemoveAll(etcdAdmConfig.InstallDir); err != nil {
			log.Print(err)
		}
		// Remove symlinks
		if err = binary.DeleteSymLinks(constants.DefaultInstallBaseDir); err != nil {
			log.Print(err)
		}
		log.Printf("[cluster] etcd reset complete")
	},
}

func init() {
	rootCmd.AddCommand(resetCmd)
	resetCmd.Flags().BoolVar(&skipRemoveMember, "skip-remove-member", constants.DefaultSkipRemoveMember, "Use skip-remove-member flag to skip the process of removing member from etcd cluster but clean everything else.")
	resetCmd.PersistentFlags().StringVar(&etcdAdmConfig.InstallBaseDir, "install-base-dir", constants.DefaultInstallBaseDir, "install base directory")
}
