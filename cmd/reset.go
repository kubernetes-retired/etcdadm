package cmd

import (
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
		// TODO: Use constants
		util.RemoveFolder("/var/lib/etcd/")
		util.RemoveFolder("/etc/etcd/pki/")
		// TODO: Remove unit file & env file

	},
}

func init() {
	rootCmd.AddCommand(resetCmd)
}
