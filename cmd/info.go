package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/platform9/etcdadm/apis"
	"github.com/platform9/etcdadm/constants"
	"github.com/platform9/etcdadm/etcd"
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

		client, err := etcd.ClientForEndpoint(etcdAdmConfig.LoopbackClientURL.String(), &etcdAdmConfig)
		if err != nil {
			log.Fatalf("[membership] Error requesting member information: %s", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultEtcdRequestTimeout)
		mresp, err := client.MemberList(ctx)
		cancel()
		if err != nil {
			log.Fatalf("[membership] Error listing members: %v", err)
		}
		localMember, ok := etcd.MemberForID(mresp.Members, mresp.Header.MemberId)
		if !ok {
			log.Fatalf("[membership] Failed to identify local member in member list")
		}
		localMemberJSON, err := json.MarshalIndent(&localMember, "", "  ")
		if err != nil {
			log.Fatalf("[info] Error parsing member information: %s", err)
		}
		fmt.Println(string(localMemberJSON))
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
}
