package controller

import (
	"context"

	"kope.io/etcd-manager/pkg/commands"
)

func (m *EtcdController) refreshCommands() error {
	// TODO: Refresh commands less frequently (maybe we need a force flag)

	commands, err := m.commandStore.ListCommands()
	if err != nil {
		return err
	}

	m.commands = commands
	return nil
}

func (m *EtcdController) getCreateNewClusterCommand() commands.Command {
	for _, c := range m.commands {
		if c.Data().CreateNewCluster != nil {
			return c
		}
	}
	return nil
}

func (m *EtcdController) getRestoreBackupCommand() commands.Command {
	for _, c := range m.commands {
		if c.Data().RestoreBackup != nil {
			return c
		}
	}
	return nil
}

func (m *EtcdController) removeCommand(ctx context.Context, cmd commands.Command) error {
	return m.commandStore.RemoveCommand(cmd)
}
