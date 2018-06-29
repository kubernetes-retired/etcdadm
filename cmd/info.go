package cmd

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/platform9/etcdadm/apis"
	"github.com/platform9/etcdadm/util"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Information about the local etcd member",
	Run: func(cmd *cobra.Command, args []string) {
		err := apis.SetJoinDynamicDefaults(&etcdAdmConfig)
		if err != nil {
			log.Fatalf("[defaults] Error: %s", err)
		}
		member, err := util.SelfMember(&etcdAdmConfig)
		if err != nil {
			log.Fatalf("[info] Error requesting member information: %s", err)
		}
		memberJSON, err := json.MarshalIndent(&member, "", "  ")
		if err != nil {
			log.Fatalf("[info] Error parsing member information: %s", err)
		}
		fmt.Println(string(memberJSON))
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
}
