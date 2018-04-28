package controller

import (
	"context"

	"kope.io/etcd-manager/pkg/backup"
)

func (m *EtcdController) refreshCommands() error {
	// TODO: Refresh commands less frequently (maybe we need a force flag)

	commands, err := m.backupStore.ListCommands()
	if err != nil {
		return err
	}

	m.commands = commands
	return nil
}

func (m *EtcdController) getCreateNewClusterCommand() *backup.Command {
	for _, c := range m.commands {
		if c.Data.CreateNewCluster != nil {
			return c
		}
	}
	return nil
}

func (m *EtcdController) getRestoreBackupCommand() *backup.Command {
	for _, c := range m.commands {
		if c.Data.RestoreBackup != nil {
			return c
		}
	}
	return nil
}

func (m *EtcdController) removeCommand(ctx context.Context, cmd *backup.Command) error {
	return m.backupStore.RemoveCommand(cmd)
}
