package controller

import (
	"context"
	"fmt"

	"github.com/golang/glog"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
)

func (m *EtcdController) restoreBackupAndLiftQuarantine(parentContext context.Context, clusterSpec *protoetcd.ClusterSpec, clusterState *etcdClusterState) (bool, error) {
	changed := false

	// We start a new context - this is pretty critical-path
	ctx := context.Background()

	var restoreRequest *protoetcd.DoRestoreRequest
	{
		backups, err := m.backupStore.ListBackups()
		if err != nil {
			return false, fmt.Errorf("error listing backups: %v", err)
		}

		for i := len(backups) - 1; i >= 0; i-- {
			backup := backups[i]

			info, err := m.backupStore.LoadInfo(backup)
			if err != nil {
				glog.Warningf("error reading backup info in %q: %v", backup, err)
				continue
			}
			if info == nil || info.EtcdVersion == "" {
				glog.Warningf("etcd version not found in %q", backup)
				continue
			} else {
				restoreRequest = &protoetcd.DoRestoreRequest{
					LeadershipToken: m.leadership.token,
					ClusterName:     m.clusterName,
					Storage:         m.backupStore.Spec(),
					BackupName:      backup,
				}
				break
			}
		}
	}

	if restoreRequest != nil {
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
		} else {
			changed = true
		}
		glog.V(2).Infof("DoRestoreResponse: %s", response)
	} else {
		// If we didn't restore a backup, we write the cluster spec to etcd
		err := m.writeClusterSpec(ctx, clusterState, clusterSpec)
		if err != nil {
			return false, err
		}
	}

	updated, err := m.updateQuarantine(ctx, clusterState, false)
	if err != nil {
		return changed, err
	}
	if updated {
		changed = true
	}
	return changed, nil
}

func (m *EtcdController) updateQuarantine(ctx context.Context, clusterState *etcdClusterState, quarantined bool) (bool, error) {
	glog.Infof("Setting quarantined state to %b", quarantined)
	changed := false
	for peerId, p := range clusterState.peers {
		member := clusterState.FindMember(peerId)
		if member == nil {
			continue
		}

		request := &protoetcd.ReconfigureRequest{
			LeadershipToken: m.leadership.token,
			ClusterName:     m.clusterName,
			Quarantined:     quarantined,
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
