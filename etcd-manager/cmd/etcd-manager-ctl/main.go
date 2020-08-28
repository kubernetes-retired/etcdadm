/*
Copyright 2017 The Kubernetes Authors.

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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/golang/protobuf/proto"
	"k8s.io/klog"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/pkg/backup"
	"kope.io/etcd-manager/pkg/commands"
)

const DefaultEtcdVersion = "3.2.24"

type Options struct {
	MemberCount     int
	EtcdVersion     string
	BackupStorePath string
}

func (o *Options) SetDefaults() {
	o.BackupStorePath = "/backups"
}

func main() {
	klog.InitFlags(nil)

	var o Options
	o.SetDefaults()

	flag.IntVar(&o.MemberCount, "member-count", o.MemberCount, "initial cluster size; cluster won't start until we have a quorum of this size")
	flag.StringVar(&o.BackupStorePath, "backup-store", o.BackupStorePath, "backup store location")
	flag.StringVar(&o.EtcdVersion, "etcd-version", o.EtcdVersion, "etcd version")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [<args>] [<command>]\n", os.Args[0])
		fmt.Print("\n\nThese are the supported args:\n\n")
		flag.PrintDefaults()
		fmt.Print("\n\nThese are the supported commands: (If no command is specified 'get' will be called.)\n\n")
		fmt.Print(`get				Shows Cluster Spec
configure-cluster		Sets cluster spec based on -member-count and -etcd-version args specified.
list-backups			List backups available in the -backup-store
list-commands			List commands in queue for cluster to execute.
delete-command			Deletes a command from the clusters queue
restore-backup			Restores the backup specified. Pass the backup timestamp shown by list-backup as parameter. 
				eg. etcd-ctl -backup-store=s3://mybackupstore/ restore-backup 2019-05-07T18:28:01Z-000977
`)
	}
	flag.Parse()
	fmt.Printf("Backup Store: %v\n", o.BackupStorePath)

	command := ""
	args := flag.Args()
	if len(args) == 0 {
		command = "get"
		args = []string{}
	} else {
		command = args[0]
		args = args[1:]
	}

	ctx := context.Background()

	if err := run(ctx, &o, command, args); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func run(ctx context.Context, o *Options, command string, args []string) error {
	switch command {
	case "get":
		return runGet(ctx, o)
	case "configure-cluster":
		return runInitCluster(ctx, o)
	case "list-backups":
		return runListBackups(ctx, o)
	case "list-commands":
		return runListCommands(ctx, o)
	case "delete-command":
		return runDeleteCommand(ctx, o, args)
	case "restore-backup":
		return runRestoreBackup(ctx, o, args)
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

func GetBackupStore(o *Options) (backup.Store, error) {
	if o.BackupStorePath == "" {
		return nil, fmt.Errorf("backup-store is required")
	}

	commandStore, err := backup.NewStore(o.BackupStorePath)
	if err != nil {
		return nil, fmt.Errorf("error initializing backup store: %v", err)
	}
	return commandStore, nil
}

func GetCommandStore(o *Options) (commands.Store, error) {
	if o.BackupStorePath == "" {
		return nil, fmt.Errorf("backup-store is required")
	}

	commandStore, err := commands.NewStore(o.BackupStorePath)
	if err != nil {
		return nil, fmt.Errorf("error initializing commands store: %v", err)
	}
	return commandStore, nil
}

func runListBackups(ctx context.Context, o *Options) error {
	backupStore, err := GetBackupStore(o)
	if err != nil {
		return err
	}

	backups, err := backupStore.ListBackups()
	if err != nil {
		return fmt.Errorf("error listing backups: %v", err)
	}

	for _, b := range backups {
		fmt.Fprintf(os.Stdout, "%v\n", b)
	}

	return nil
}

func runListCommands(ctx context.Context, o *Options) error {
	commandStore, err := GetCommandStore(o)
	if err != nil {
		return err
	}

	commands, err := commandStore.ListCommands()
	if err != nil {
		return fmt.Errorf("error listing commands: %v", err)
	}

	for _, c := range commands {
		data := c.Data()
		fmt.Fprintf(os.Stdout, "%s\n", proto.CompactTextString(&data))
	}

	return nil
}

func runDeleteCommand(ctx context.Context, o *Options, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("syntax: delete-command <backupname>")
	}
	commandID := args[0]

	commandStore, err := GetCommandStore(o)
	if err != nil {
		return err
	}

	commands, err := commandStore.ListCommands()
	if err != nil {
		return fmt.Errorf("error listing commands: %v", err)
	}

	for _, c := range commands {
		data := c.Data()
		if strconv.FormatInt(data.Timestamp, 10) == commandID {
			err := commandStore.RemoveCommand(c)
			if err != nil {
				return fmt.Errorf("error deleting command: %v", err)
			}
		}
	}

	return nil
}

func runRestoreBackup(ctx context.Context, o *Options, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("syntax: restore-backup <backupname>")
	}
	backupName := args[0]

	commandStore, err := GetCommandStore(o)
	if err != nil {
		return err
	}

	// Start with current configuration
	clusterSpec, err := commandStore.GetExpectedClusterSpec()
	if err != nil {
		return fmt.Errorf("error reading etcd cluster spec: %v", err)
	}

	if clusterSpec == nil {
		return fmt.Errorf("cluster spec was not found - is this a new cluster?")
	}

	cmd := &protoetcd.Command{
		RestoreBackup: &protoetcd.RestoreBackupCommand{
			Backup:      backupName,
			ClusterSpec: clusterSpec,
		},
	}

	if err := commandStore.AddCommand(cmd); err != nil {
		return fmt.Errorf("error writing command to store: %v", err)
	}

	fmt.Fprintf(os.Stdout, "added restore-backup command: %v\n", cmd)
	return nil
}

func runInitCluster(ctx context.Context, o *Options) error {
	commandStore, err := GetCommandStore(o)
	if err != nil {
		return err
	}

	// Start with current configuration
	clusterSpec, err := commandStore.GetExpectedClusterSpec()
	if err != nil {
		return fmt.Errorf("error reading etcd cluster spec: %v", err)
	}

	if clusterSpec == nil {
		clusterSpec = &protoetcd.ClusterSpec{}
	}

	// Populate values specified by user
	if o.MemberCount != 0 {
		clusterSpec.MemberCount = int32(o.MemberCount)
	}
	if o.EtcdVersion != "" {
		clusterSpec.EtcdVersion = o.EtcdVersion
	}

	// Populate default values
	if clusterSpec.EtcdVersion == "" {
		clusterSpec.EtcdVersion = DefaultEtcdVersion
	}
	if clusterSpec.MemberCount == 0 {
		clusterSpec.MemberCount = 1
	}

	if err := commandStore.SetExpectedClusterSpec(clusterSpec); err != nil {
		return fmt.Errorf("error setting expected cluster spec: %v", err)
	}

	fmt.Fprintf(os.Stdout, "set cluster spec to %v\n", clusterSpec)

	return nil
}

func runGet(ctx context.Context, o *Options) error {
	commandStore, err := GetCommandStore(o)
	if err != nil {
		return err
	}

	clusterSpec, err := commandStore.GetExpectedClusterSpec()
	if err != nil {
		return fmt.Errorf("error reading etcd cluster spec: %v", err)
	}

	if clusterSpec == nil {
		return fmt.Errorf("no spec found in store %q (consider calling etcd-manager configure-cluster)", o.BackupStorePath)
	}
	fmt.Fprintf(os.Stdout, "%v\n", clusterSpec)

	return nil
}
