package cmd

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"

	apimachineryversion "k8s.io/apimachinery/pkg/version"
	"k8s.io/kubernetes/pkg/version"
)

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
