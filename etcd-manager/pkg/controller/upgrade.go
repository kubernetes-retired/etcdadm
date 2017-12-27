package controller

import (
	"context"
	"fmt"

	"github.com/golang/glog"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
)

func (m *EtcdController) stopForUpgrade(parentContext context.Context, clusterSpec *protoetcd.ClusterSpec, clusterState *etcdClusterState) (bool, error) {
	// We start a new context - this is pretty critical-path
	ctx := context.Background()

	// Force a backup, even before we start to do anything
	if _, err := m.doClusterBackup(ctx, clusterSpec, clusterState); err != nil {
		return false, fmt.Errorf("error doing backup before upgrade/downgrade: %v", err)
	}

	memberToPeer := make(map[EtcdMemberId]*peer)
	for memberId, member := range clusterState.members {
		found := false
		for _, p := range clusterState.peers {
			if p.info == nil || p.info.NodeConfiguration == nil || p.peer == nil {
				continue
			}
			if p.info.NodeConfiguration.Name == string(member.Name) {
				memberToPeer[memberId] = p.peer
				found = true
			}
		}
		if !found {
			return false, fmt.Errorf("unable to find peer for %q", member.Name)
		}

		if len(clusterState.healthyMembers) != len(clusterState.members) {
			// This one seems hard to relax
			return false, fmt.Errorf("cannot upgrade/downgrade cluster when not all members are healthy")
		}

		if len(clusterState.members) != int(clusterSpec.MemberCount) {
			// We could relax this, but we probably don't want to
			return false, fmt.Errorf("cannot upgrade/downgrade cluster when cluster is not at full member count")
		}

		// Stop the whole cluster
		var lastStopped *peer
		for memberId := range clusterState.members {
			peer := memberToPeer[memberId]
			if peer == nil {
				// We checked this when we built the map
				panic("peer unexpectedly not found - logic error")
			}

			request := &protoetcd.StopEtcdRequest{
				ClusterName:     m.clusterName,
				LeadershipToken: m.leadership.token,
			}
			response, err := peer.rpcStopEtcd(ctx, request)
			if err != nil {
				return false, fmt.Errorf("error stopping etcd peer %q: %v", peer.Id, err)
			}
			glog.Infof("stopped etcd on peer %q: %v", peer.Id, response)
			lastStopped = peer
		}

		// Do a backup on the peer we stopped last (in a 3 node cluster, we might still commit after we stop the first node)
		var backupResponse *protoetcd.DoBackupResponse
		{
			info := &protoetcd.BackupInfo{
				ClusterSpec: clusterSpec,
			}
			request := &protoetcd.DoBackupRequest{
				LeadershipToken: m.leadership.token,
				ClusterName:     m.clusterName,
				Storage:         m.backupStore.Spec(),
				Info:            info,
			}

			var err error
			backupResponse, err = lastStopped.rpcDoBackup(ctx, request)
			if err != nil {
				// TODO: inject faults here during testing
				return false, fmt.Errorf("failed to do backup: %v", err)
			} else {
				glog.V(2).Infof("backup response: %v", backupResponse)
			}
		}

		//// Start a new cluster, using the backup
		//{
		//	clusterToken := randomToken()
		//
		//	var proposedNodes []*protoetcd.EtcdNode
		//	for _, p := range clusterState.members {
		//		node := &protoetcd.EtcdNode{
		//			Name:        p.Name,
		//			PeerUrls:    p.PeerURLs,
		//			ClientUrls:  p.ClientURLs,
		//			EtcdVersion: clusterSpec.EtcdVersion,
		//		}
		//		proposedNodes = append(proposedNodes, nodes)
		//	}
		//
		//	for _, member := range clusterState.members {
		//		peer := memberToPeer[EtcdMemberId(member.Id)]
		//		if peer == nil {
		//			// We checked this when we built the map
		//			panic("peer unexpectedly not found - logic error")
		//		}
		//
		//		joinClusterRequest := &protoetcd.JoinClusterRequest{
		//			LeadershipToken: m.leadership.token,
		//			Phase:           protoetcd.Phase_PHASE_PREPARE,
		//			ClusterName:     m.clusterName,
		//			ClusterToken:    clusterToken,
		//			Nodes:           proposedNodes,
		//		}
		//
		//		joinClusterResponse, err := peer.rpcJoinCluster(ctx, joinClusterRequest)
		//		if err != nil {
		//			// TODO: Send a CANCEL message for anything PREPAREd?
		//			return false, fmt.Errorf("error from JoinClusterRequest from peer %q: %v", peer, err)
		//		}
		//		glog.V(2).Infof("JoinClusterResponse: %s", joinClusterResponse)
		//	}
		//
		//	for _, member := range clusterState.members {
		//		peer := memberToPeer[EtcdMemberId(member.Id)]
		//		if peer == nil {
		//			// We checked this when we built the map
		//			panic("peer unexpectedly not found - logic error")
		//		}
		//		// Note the we send the message to ourselves
		//		joinClusterRequest := &protoetcd.JoinClusterRequest{
		//			LeadershipToken: m.leadership.token,
		//			Phase:           protoetcd.Phase_PHASE_INITIAL_CLUSTER,
		//			ClusterName:     m.clusterName,
		//			ClusterToken:    clusterToken,
		//			Nodes:           proposedNodes,
		//		}
		//
		//		joinClusterResponse, err := peer.rpcJoinCluster(ctx, joinClusterRequest)
		//		if err != nil {
		//			// TODO: Send a CANCEL message for anything PREPAREd?
		//			return false, fmt.Errorf("error from JoinClusterRequest from peer %q: %v", p.peer, err)
		//		}
		//		glog.V(2).Infof("JoinClusterResponse: %s", joinClusterResponse)
		//	}
		//}
		//
		//// We wait for up to 5 minutes for our new cluster
	}

	// TODO: Enforce the upgrade sequence 2.2.1 2.3.7 3.0.17 3.1.11
	// TODO: The 2 -> 3 upgrade is a dump anyway

	return true, nil
}
