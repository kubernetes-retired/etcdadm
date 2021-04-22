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
	"time"

	"k8s.io/klog/v2"
	protoetcd "sigs.k8s.io/etcdadm/etcd-manager/pkg/apis/etcd"
)

func (m *EtcdController) prepareForUpgrade(clusterSpec *protoetcd.ClusterSpec, clusterState *etcdClusterState) (map[EtcdMemberId]*etcdClusterPeerInfo, error) {
	// Sanity check
	memberToPeer := make(map[EtcdMemberId]*etcdClusterPeerInfo)
	for memberId, member := range clusterState.members {
		found := false
		for _, p := range clusterState.peers {
			if p.info == nil || p.info.NodeConfiguration == nil || p.peer == nil {
				continue
			}
			if p.info.NodeConfiguration.Name == string(member.Name) {
				memberToPeer[memberId] = p
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("unable to find peer for %q", member.Name)
		}
	}

	if len(clusterState.healthyMembers) != len(clusterState.members) {
		// This one seems hard to relax
		return nil, fmt.Errorf("cannot upgrade/downgrade cluster when not all members are healthy")
	}

	if len(clusterState.members) != int(clusterSpec.MemberCount) {
		// We could relax this, but we probably don't want to
		return nil, fmt.Errorf("cannot upgrade/downgrade cluster when cluster is not at full member count")
	}

	return memberToPeer, nil
}

func (m *EtcdController) stopForUpgrade(parentContext context.Context, clusterSpec *protoetcd.ClusterSpec, clusterState *etcdClusterState) (bool, error) {
	// We start a new context - this is pretty critical-path
	ctx := context.Background()

	memberToPeer, err := m.prepareForUpgrade(clusterSpec, clusterState)
	if err != nil {
		return false, err
	}

	// Force a backup, even before we start to do anything
	klog.Infof("backing up cluster before quarantine")
	if _, err := m.doClusterBackup(ctx, clusterSpec, clusterState); err != nil {
		return false, fmt.Errorf("error doing backup before upgrade/downgrade: %v", err)
	}

	// We quarantine first, so that we don't have to get down to a single node before it is safe to do a backup
	klog.Infof("quarantining cluster for upgrade")
	if _, err := m.updateQuarantine(ctx, clusterState, true); err != nil {
		return false, err
	}

	// We do a backup
	klog.Infof("backing up cluster after quarantine")
	backupResponse, err := m.doClusterBackup(ctx, clusterSpec, clusterState)
	if err != nil {
		return false, err
	}
	klog.Infof("backed up cluster as %v", backupResponse)

	// Schedule a restore of the backup onto the new cluster
	{
		cmd := &protoetcd.Command{
			RestoreBackup: &protoetcd.RestoreBackupCommand{
				Backup:      backupResponse.Name,
				ClusterSpec: clusterSpec,
			},
		}

		if err := m.controlStore.AddCommand(cmd); err != nil {
			return false, fmt.Errorf("error adding restore command: %v", err)
		}

		for {
			if err := m.refreshControlStore(time.Duration(0)); err != nil {
				return false, fmt.Errorf("error refreshing commands: %v", err)
			}
			if m.getRestoreBackupCommand() != nil {
				break
			}
			klog.Warningf("waiting for restore-backup command to be visible")
			time.Sleep(5 * time.Second)
		}
	}

	// Stop the whole cluster
	for memberId := range clusterState.members {
		peer := memberToPeer[memberId]
		if peer == nil {
			// We checked this when we built the map
			panic("peer unexpectedly not found - logic error")
		}

		request := &protoetcd.StopEtcdRequest{
			Header: m.buildHeader(),
		}
		response, err := peer.peer.rpcStopEtcd(ctx, request)
		if err != nil {
			return false, fmt.Errorf("error stopping etcd peer %q: %v", peer.peer.Id, err)
		}
		klog.Infof("stopped etcd on peer %q: %v", peer.peer.Id, response)
	}

	return true, nil
}

func (m *EtcdController) upgradeInPlace(parentContext context.Context, clusterSpec *protoetcd.ClusterSpec, clusterState *etcdClusterState) (bool, error) {
	// We start a new context - this is pretty critical-path
	ctx := context.Background()

	klog.Infof("doing in-place upgrade to %q", clusterSpec.EtcdVersion)

	memberToPeer, err := m.prepareForUpgrade(clusterSpec, clusterState)
	if err != nil {
		return false, err
	}

	for memberId := range clusterState.members {
		p := memberToPeer[memberId]
		if p == nil {
			// We checked this when we built the map
			panic("peer unexpectedly not found - logic error")
		}

		if p.info == nil || p.info.EtcdState == nil || p.peer == nil {
			return false, fmt.Errorf("cannot upgrade; insufficient info for peer %q", p.peer.Id)
		}
	}

	// We do a backup
	klog.Infof("backing up cluster before in-place upgrade")
	backupResponse, err := m.doClusterBackup(ctx, clusterSpec, clusterState)
	if err != nil {
		return false, err
	}
	klog.Infof("backup done: %v", backupResponse)

	for memberId := range clusterState.members {
		peer := memberToPeer[memberId]
		if peer == nil {
			// We checked this when we built the map
			panic("peer unexpectedly not found - logic error")
		}

		if peer.info.EtcdState.EtcdVersion == clusterSpec.EtcdVersion {
			continue
		}

		klog.Infof("reconfiguring peer %q from %q -> %q", peer.peer.Id, peer.info.EtcdState.EtcdVersion, clusterSpec.EtcdVersion)

		request := &protoetcd.ReconfigureRequest{
			Header:         m.buildHeader(),
			Quarantined:    peer.info.EtcdState.Quarantined,
			SetEtcdVersion: clusterSpec.EtcdVersion,
		}

		response, err := peer.peer.rpcReconfigure(ctx, request)
		if err != nil {
			return false, fmt.Errorf("error reconfiguring etcd peer %q: %v", peer.peer.Id, err)
		}
		klog.Infof("reconfigured etcd on peer %q: %v", peer.peer.Id, response)

		// We run one node per cycle so we can be sure we are fault tolerant
		return true, nil
	}

	klog.Warningf("no nodes were upgraded")
	return false, nil
}
