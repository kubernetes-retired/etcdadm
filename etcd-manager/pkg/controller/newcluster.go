package controller

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
)

// createNewCluster starts a new etcd cluster.
// It tries to identify a quorum of nodes, and if found will instruct each to join the cluster.
func (m *EtcdController) createNewCluster(ctx context.Context, clusterState *etcdClusterState, clusterSpec *protoetcd.ClusterSpec) (bool, error) {
	desiredMemberCount := int(clusterSpec.MemberCount)
	desiredQuorumSize := quorumSize(desiredMemberCount)

	if len(clusterState.peers) < desiredQuorumSize {
		glog.Infof("Insufficient peers to form a quorum %d, won't proceed", desiredQuorumSize)
		return false, nil
	}

	if len(clusterState.peers) < desiredMemberCount {
		// TODO: We should relax this, but that requires etcd to support an explicit quorum setting, or we can create dummy entries

		// But ... as a special case, we can allow it through if the quorum size is the same (i.e. one less than desired)
		if quorumSize(len(clusterState.peers)) == desiredQuorumSize {
			glog.Infof("Fewer peers (%d) than desired members (%d), but quorum size is the same, so will proceed", len(clusterState.peers), desiredMemberCount)
		} else {
			glog.Infof("Insufficient peers to form full cluster %d, won't proceed", desiredQuorumSize)
			return false, nil
		}
	}

	clusterToken := randomToken()

	var proposal []*etcdClusterPeerInfo
	for _, peer := range clusterState.peers {
		proposal = append(proposal, peer)
		if len(proposal) == desiredMemberCount {
			// We have identified enough members to form a cluster
			break
		}
	}

	if len(proposal) < desiredMemberCount && quorumSize(len(proposal)) < quorumSize(desiredMemberCount) {
		glog.Fatalf("Need to add dummy peers to force quorum size :-(")
	}

	// Build the proposed nodes and the proposed member map

	var proposedNodes []*protoetcd.EtcdNode
	memberMap := &protoetcd.MemberMap{}
	for _, p := range proposal {
		node := proto.Clone(p.info.NodeConfiguration).(*protoetcd.EtcdNode)
		proposedNodes = append(proposedNodes, node)

		memberInfo := &protoetcd.MemberMapInfo{
			Name: node.Name,
		}

		if m.dnsSuffix != "" {
			dnsSuffix := m.dnsSuffix
			if !strings.HasPrefix(dnsSuffix, ".") {
				dnsSuffix = "." + dnsSuffix
			}
			memberInfo.Dns = node.Name + dnsSuffix
		}

		if p.peer.info == nil {
			return false, fmt.Errorf("no info for peer %v", p)
		}

		for _, a := range p.peer.info.Endpoints {
			ip := a
			colonIndex := strings.Index(ip, ":")
			if colonIndex != -1 {
				ip = ip[:colonIndex]
			}
			memberInfo.Addresses = append(memberInfo.Addresses, ip)
		}

		memberMap.Members = append(memberMap.Members, memberInfo)
	}

	// Stop any running etcd
	for _, p := range clusterState.peers {
		peer := p.peer

		if p.info != nil && p.info.EtcdState != nil {
			request := &protoetcd.StopEtcdRequest{
				Header: m.buildHeader(),
			}
			response, err := peer.rpcStopEtcd(ctx, request)
			if err != nil {
				return false, fmt.Errorf("error stopping etcd peer %q: %v", peer.Id, err)
			}
			glog.Infof("stopped etcd on peer %q: %v", peer.Id, response)
		}
	}

	// Broadcast the proposed member map so everyone is consistent
	if errors := m.broadcastMemberMap(ctx, clusterState, memberMap); len(errors) != 0 {
		return false, fmt.Errorf("unable to broadcast member map: %v", errors)
	}

	glog.Infof("starting new etcd cluster with %s", proposal)

	for _, p := range proposal {
		// Note the we may send the message to ourselves
		joinClusterRequest := &protoetcd.JoinClusterRequest{
			Header:       m.buildHeader(),
			Phase:        protoetcd.Phase_PHASE_PREPARE,
			ClusterToken: clusterToken,
			EtcdVersion:  clusterSpec.EtcdVersion,
			Nodes:        proposedNodes,
		}

		joinClusterResponse, err := p.peer.rpcJoinCluster(ctx, joinClusterRequest)
		if err != nil {
			// TODO: Send a CANCEL message for anything PREPAREd?  (currently we rely on a slow timeout)
			return false, fmt.Errorf("error from JoinClusterRequest (prepare) from peer %q: %v", p.peer, err)
		}
		glog.V(2).Infof("JoinClusterResponse: %s", joinClusterResponse)
	}

	for _, p := range proposal {
		// Note the we may send the message to ourselves
		joinClusterRequest := &protoetcd.JoinClusterRequest{
			Header:       m.buildHeader(),
			Phase:        protoetcd.Phase_PHASE_INITIAL_CLUSTER,
			ClusterToken: clusterToken,
			EtcdVersion:  clusterSpec.EtcdVersion,
			Nodes:        proposedNodes,
		}

		joinClusterResponse, err := p.peer.rpcJoinCluster(ctx, joinClusterRequest)
		if err != nil {
			// TODO: Send a CANCEL message for anything PREPAREd?  (currently we rely on a slow timeout)
			return false, fmt.Errorf("error from JoinClusterRequest from peer %q: %v", p.peer, err)
		}
		glog.V(2).Infof("JoinClusterResponse: %s", joinClusterResponse)
	}

	// Write cluster spec to etcd
	if err := m.writeClusterSpec(ctx, clusterState, clusterSpec); err != nil {
		return false, err
	}

	return true, nil
}
