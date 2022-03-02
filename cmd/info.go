/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	log "sigs.k8s.io/etcdadm/pkg/logrus"

	"github.com/spf13/cobra"
	"sigs.k8s.io/etcdadm/apis"
	"sigs.k8s.io/etcdadm/constants"
	"sigs.k8s.io/etcdadm/etcd"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Information about the local etcd member",
	Run: func(cmd *cobra.Command, args []string) {
		err := apis.SetJoinDynamicDefaults(&etcdAdmConfig)
		if err != nil {
			log.Fatalf("[defaults] Error: %s", err)
		}

		client, err := etcd.ClientForEndpoints([]string{etcdAdmConfig.LoopbackClientURL.String()}, &etcdAdmConfig)
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
