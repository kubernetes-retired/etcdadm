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
	"encoding/json"
	"fmt"

	log "sigs.k8s.io/etcdadm/pkg/logrus"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	apimachineryversion "k8s.io/apimachinery/pkg/version"
	"k8s.io/component-base/version"
)

// Version TODO: add description
type Version struct {
	ClientVersion *apimachineryversion.Info `json:"clientVersion,omitempty"`
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		short, err := cmd.Flags().GetBool("short")
		if err != nil {
			log.Fatalf("Error parsing option value for short")
		}
		output, err := cmd.Flags().GetString("output")
		if err != nil {
			log.Fatalf("Error parsing option value for output")
		}

		var versionInfo Version
		clientVersion := version.Get()
		versionInfo.ClientVersion = &clientVersion

		switch output {
		case "":
			if short {
				fmt.Println(clientVersion.GitVersion)
			} else {
				fmt.Println(fmt.Sprintf("%#v", clientVersion))
			}
		case "yaml":
			marshalled, err := yaml.Marshal(&versionInfo)
			if err != nil {
				log.Fatalf("Error encoding version information as yaml")
			}
			fmt.Println(string(marshalled))
		case "json":
			marshalled, err := json.MarshalIndent(&versionInfo, "", "  ")
			if err != nil {
				log.Fatalf("Error encoding version information as json")
			}
			fmt.Println(string(marshalled))
		default:
			log.Fatalf("Invalid output format. Use yaml/json")
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	versionCmd.Flags().Bool("short", false, "Set true for short format")
	versionCmd.Flags().String("output", "", "Specify output format yaml/json")
}
