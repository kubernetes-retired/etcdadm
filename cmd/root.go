package cmd

import (
	"fmt"
	"os"

	"github.com/platform9/etcdadm/apis"
	"github.com/spf13/cobra"
)

var etcdAdmConfig apis.EtcdAdmConfig

var (
	rootCmd = &cobra.Command{
		Use:  "etcdadm",
		Long: `Tool to bootstrap etcdadm on the host`,
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
