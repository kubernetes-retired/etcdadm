package controller

import (
	"context"
	"time"

	"k8s.io/klog"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/pkg/commands"
)

func (m *EtcdController) InvalidateControlStore() error {
	return m.refreshControlStore(time.Duration(0))
}

func (m *EtcdController) refreshControlStore(ttl time.Duration) error {
	m.controlMutex.Lock()
	defer m.controlMutex.Unlock()

	now := time.Now()
	if ttl != time.Duration(0) && now.Before(m.controlLastRead.Add(ttl)) {
		klog.V(4).Infof("not refreshing commands - TTL not hit")
		return nil
	}
	klog.Infof("refreshing commands")
	controlCommands, err := m.controlStore.ListCommands()
	if err != nil {
		return err
	}
	controlClusterSpec, err := m.controlStore.GetExpectedClusterSpec()
	if err != nil {
		return err
	}

	m.controlCommands = controlCommands
	m.controlLastRead = now
	m.controlClusterSpec = controlClusterSpec

	return nil
}

func (m *EtcdController) getControlClusterSpec() *protoetcd.ClusterSpec {
	m.controlMutex.Lock()
	defer m.controlMutex.Unlock()

	return m.controlClusterSpec
}

func (m *EtcdController) getRestoreBackupCommand() commands.Command {
	m.controlMutex.Lock()
	defer m.controlMutex.Unlock()

	for _, c := range m.controlCommands {
		if c.Data().RestoreBackup != nil {
			return c
		}
	}
	return nil
}

func (m *EtcdController) removeCommand(ctx context.Context, cmd commands.Command) error {
	m.controlMutex.Lock()
	defer m.controlMutex.Unlock()

	err := m.controlStore.RemoveCommand(cmd)

	m.controlCommands = nil
	m.controlLastRead = time.Time{}

	return err
}
