/*
Copyright 2020 The Kubernetes Authors.

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

package controller

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/proto"
	"k8s.io/klog"

	protoetcd "sigs.k8s.io/etcdadm/etcd-manager/pkg/apis/etcd"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/commands"
)

func (m *EtcdController) restoreBackupAndLiftQuarantine(parentContext context.Context, clusterSpec *protoetcd.ClusterSpec, clusterState *etcdClusterState, cmd commands.Command) (bool, error) {
	// We start a new context - this is pretty critical-path
	ctx := context.Background()

	if cmd.Data().RestoreBackup == nil || cmd.Data().RestoreBackup.Backup == "" {
		// Should be unreachable
		return false, fmt.Errorf("RestoreBackup not set in %v", cmd)
	}

	backup := cmd.Data().RestoreBackup.Backup

	info, err := m.backupStore.LoadInfo(backup)
	if err != nil {
		return false, fmt.Errorf("error reading backup info in %q: %v", backup, err)
	}
	if info == nil || info.EtcdVersion == "" {
		return false, fmt.Errorf("etcd version not found in %q", backup)
	}
	restoreRequest := &protoetcd.DoRestoreRequest{
		Header:     m.buildHeader(),
		Storage:    m.backupStore.Spec(),
		BackupName: backup,
	}

	var peer *etcdClusterPeerInfo
	for peerId, p := range clusterState.peers {
		member := clusterState.FindHealthyMember(peerId)
		if member == nil {
			continue
		}
		peer = p
	}
	if peer == nil {
		return false, fmt.Errorf("unable to find peer on which to run restore operation")
	}

	response, err := peer.peer.rpcDoRestore(ctx, restoreRequest)
	if err != nil {
		return false, fmt.Errorf("error restoring backup on peer %v: %v", peer.peer, err)
	}
	klog.V(2).Infof("DoRestoreResponse: %s", response)

	// Write cluster spec to etcd - we may have written over it with the restore
	{
		newClusterSpec := proto.Clone(clusterSpec).(*protoetcd.ClusterSpec)

		if err := m.writeClusterSpec(ctx, clusterState, newClusterSpec); err != nil {
			return false, err
		}
	}

	// Remove command so we won't restore again
	if err := m.removeCommand(ctx, cmd); err != nil {
		return false, err
	}

	return true, nil
}

// writeClusterSpec writes the cluster spec into the control store
func (m *EtcdController) writeClusterSpec(ctx context.Context, clusterState *etcdClusterState, newClusterSpec *protoetcd.ClusterSpec) error {
	if err := m.controlStore.SetExpectedClusterSpec(newClusterSpec); err != nil {
		return err
	}
	return nil
}

func (m *EtcdController) updateQuarantine(ctx context.Context, clusterState *etcdClusterState, quarantined bool) (bool, error) {
	klog.Infof("Setting quarantined state to %t", quarantined)
	changed := false
	for peerId, p := range clusterState.peers {
		member := clusterState.FindMember(peerId)
		if member == nil {
			continue
		}

		request := &protoetcd.ReconfigureRequest{
			Header:      m.buildHeader(),
			Quarantined: quarantined,
		}

		response, err := p.peer.rpcReconfigure(ctx, request)
		if err != nil {
			return changed, fmt.Errorf("error reconfiguring peer %v to not be quarantined: %v", p.peer, err)
		}
		changed = true
		klog.V(2).Infof("ReconfigureResponse: %s", response)
	}
	return changed, nil
}
