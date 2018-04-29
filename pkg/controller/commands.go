package controller

import (
	"context"
	"time"

	"github.com/golang/glog"
	"kope.io/etcd-manager/pkg/commands"
)

func (m *EtcdController) refreshCommands(ttl time.Duration) error {
	now := time.Now()
	if now.Before(m.commandsLastRead.Add(ttl)) {
		glog.V(4).Infof("not refreshing commands - TTL not hit")
		return nil
	}
	glog.Infof("refreshing commands")
	commands, err := m.commandStore.ListCommands()
	if err != nil {
		return err
	}
	m.commands = commands
	m.commandsLastRead = now
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
	err := m.commandStore.RemoveCommand(cmd)

	m.commands = nil
	m.commandsLastRead = time.Time{}

	return err
}
