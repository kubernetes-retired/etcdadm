package cmd

import (
	"github.com/platform9/etcdadm/constants"
	"github.com/platform9/etcdadm/util"
	"github.com/spf13/cobra"
)

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset a new etcd cluster",
	Run: func(cmd *cobra.Command, args []string) {
		// Remove self as member from etcd cluster
		util.RemoveSelfFromEtcdCluster(&etcdAdmConfig)

		// Remove etcd datastore
		util.RemoveFolderRecursive(constants.DefaultDataDir)

		// Remove configuration files
		util.RemoveFolderRecursive(constants.DefaultCertificateDir)
		util.RemoveFile(constants.UnitFile)
		util.RemoveFile(constants.EnvironmentFile)
		util.RemoveFile(constants.EtcdctlEnvFile)
	},
}

func init() {
	rootCmd.AddCommand(resetCmd)
}
