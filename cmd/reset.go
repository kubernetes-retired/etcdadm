package cmd

import (
	"log"
	"path/filepath"

	"github.com/platform9/etcdadm/apis"
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
		err = apis.SetJoinDynamicDefaults(&etcdAdmConfig)
		if err != nil {
			log.Fatalf("[defaults] Error: %s", err)
		}
		// Remove self as member from etcd cluster
		if skipRemoveMember == false {
			err = util.RemoveSelfFromEtcdCluster(&etcdAdmConfig)
			if err != nil {
				log.Fatal(err)
			}
		}
		// Remove etcd datastore
		util.RemoveFolderRecursive(constants.DefaultDataDir)
		// Disable and stop etcd service
		unit := filepath.Base(constants.UnitFile)
		service.DisableAndStopService(unit)
		// Remove configuration files
		util.RemoveFolderRecursive(constants.DefaultCertificateDir)
		util.RemoveFile(constants.UnitFile)
		util.RemoveFile(constants.EnvironmentFile)
		util.RemoveFile(constants.EtcdctlEnvFile)
		log.Printf("[cluster] etcd reset complete")
	},
}

func init() {
	rootCmd.AddCommand(resetCmd)
	resetCmd.Flags().BoolVar(&skipRemoveMember, "skip-remove-member", false, "Use skip-remove-member flag to skip the process of removing member from etcd cluster but clean everything else.")
}
