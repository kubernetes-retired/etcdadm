package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"

	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/pkg/backup"
)

func (m *EtcdController) restoreBackupAndLiftQuarantine(parentContext context.Context, clusterSpec *protoetcd.ClusterSpec, clusterState *etcdClusterState, cmd *backup.Command) (bool, error) {
	// We start a new context - this is pretty critical-path
	ctx := context.Background()

	if cmd.Data.RestoreBackup == nil || cmd.Data.RestoreBackup.Backup == "" {
		// Should be unreachable
		return false, fmt.Errorf("RestoreBackup not set in %v", cmd)
	}

	backup := cmd.Data.RestoreBackup.Backup

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
	glog.V(2).Infof("DoRestoreResponse: %s", response)

	// Write cluster spec to etcd - we may have written over it with the restore
	{
		newClusterSpec := proto.Clone(clusterSpec).(*protoetcd.ClusterSpec)

		if err := m.writeClusterSpecAfterRestart(ctx, clusterState, newClusterSpec); err != nil {
			return false, err
		}
	}

	// Remove command so we won't restore again
	if err := m.removeCommand(ctx, cmd); err != nil {
		return false, err
	}

	return true, nil
}

// writeClusterSpecAfterRestart waits for the cluster to be ready, and writes the updated cluster spec to it
func (m *EtcdController) writeClusterSpecAfterRestart(ctx context.Context, clusterState *etcdClusterState, newClusterSpec *protoetcd.ClusterSpec) error {
	deadline := time.Now().Add(time.Minute)

	var lastErr error

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("error getting cluster state after cluster start: %v", lastErr)
		} else {
			glog.Warningf("waiting to get cluster state after cluster restart: %v", lastErr)
		}

		time.Sleep(2 * time.Second)

		var peerList []*peer
		for _, p := range clusterState.peers {
			peerList = append(peerList, p.peer)
		}
		newClusterState, err := m.updateClusterState(ctx, peerList)
		if err != nil {
			lastErr = err
			continue
		}

		// readyCount := 0
		// for _, member := range newClusterState.members {
		// 	if len(member.ClientURLs) > 0 {
		// 		readyCount++
		// 	}
		// }

		// if readyCount == 0 {
		// 	lastErr = fmt.Errorf("cluster members were not ready")
		// 	continue
		// }

		if err := m.writeClusterSpec(ctx, newClusterState, newClusterSpec); err != nil {
			lastErr = err
			continue
		}

		return nil
	}
}

func (m *EtcdController) updateQuarantine(ctx context.Context, clusterState *etcdClusterState, quarantined bool) (bool, error) {
	glog.Infof("Setting quarantined state to %t", quarantined)
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
		glog.V(2).Infof("ReconfigureResponse: %s", response)
	}
	return changed, nil
}
