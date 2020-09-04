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
	"bytes"
	"context"
	"crypto/tls"
	"fmt"

	"k8s.io/klog"
	protoetcd "sigs.k8s.io/etcdadm/etcd-manager/pkg/apis/etcd"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/etcdclient"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/privateapi"
)

type EtcdMemberId string

type etcdClusterState struct {
	// etcdClientTLSConfig is the tls.Config for connecting to etcd as a client
	etcdClientTLSConfig *tls.Config

	members        map[EtcdMemberId]*etcdclient.EtcdProcessMember
	peers          map[privateapi.PeerId]*etcdClusterPeerInfo
	healthyMembers map[EtcdMemberId]*etcdclient.EtcdProcessMember
}

func (s *etcdClusterState) FindMember(peerId privateapi.PeerId) *etcdclient.EtcdProcessMember {
	for _, member := range s.members {
		if member.Name == string(peerId) {
			return member
		}
	}
	return nil
}

func (s *etcdClusterState) FindHealthyMember(peerId privateapi.PeerId) *etcdclient.EtcdProcessMember {
	for _, member := range s.healthyMembers {
		if member.Name == string(peerId) {
			return member
		}
	}
	return nil
}

func (s *etcdClusterState) FindPeer(member *etcdclient.EtcdProcessMember) *etcdClusterPeerInfo {
	for peerId, peer := range s.peers {
		if member.Name == string(peerId) {
			return peer
		}
	}
	return nil
}

func (s *etcdClusterState) String() string {
	var b bytes.Buffer

	fmt.Fprintf(&b, "etcdClusterState\n")
	fmt.Fprintf(&b, "  members:\n")
	for id, m := range s.members {
		fmt.Fprintf(&b, "    %s\n", m)
		if s.healthyMembers[id] == nil {
			fmt.Fprintf(&b, "      NOT HEALTHY\n")
		}
	}
	fmt.Fprintf(&b, "  peers:\n")
	for _, m := range s.peers {
		fmt.Fprintf(&b, "    %s\n", m)
	}
	return b.String()
}

type etcdClusterPeerInfo struct {
	peer *peer
	info *protoetcd.GetInfoResponse
}

func (p *etcdClusterPeerInfo) String() string {
	return fmt.Sprintf("etcdClusterPeerInfo{peer=%s, info=%s}", p.peer, p.info)
}

// newEtcdClient builds a client for the speicfied member.  We do this
// because clientURLs as reported by etcd might not be correct,
// because it's ultimately controlled by command line flags, and does
// not go through raft.
func (s *etcdClusterState) newEtcdClient(member *etcdclient.EtcdProcessMember) (etcdclient.EtcdClient, error) {
	clientURLs := member.ClientURLs

	var node *protoetcd.GetInfoResponse
	for _, p := range s.peers {
		if p.info == nil || p.info.NodeConfiguration == nil {
			continue
		}
		if p.info.NodeConfiguration.Name == member.Name {
			node = p.info
		}
	}
	if node == nil {
		klog.Warningf("unable to find node for member %q; using default clientURLs %v", member.Name, clientURLs)
	} else if node.NodeConfiguration == nil {
		klog.Warningf("unable to find node configuration for member %q; using default clientURLs %v", member.Name, clientURLs)
	} else {
		clientURLs = node.NodeConfiguration.ClientUrls
		if node.EtcdState != nil && node.EtcdState.Quarantined {
			clientURLs = node.NodeConfiguration.QuarantinedClientUrls
		}
	}

	etcdClient, err := member.NewClient(clientURLs, s.etcdClientTLSConfig)
	return etcdClient, err
}

func (s *etcdClusterState) etcdAddMember(ctx context.Context, nodeInfo *protoetcd.EtcdNode) (*etcdclient.EtcdProcessMember, error) {
	for _, member := range s.members {
		etcdClient, err := s.newEtcdClient(member)
		if err != nil {
			klog.Warningf("unable to build client for member %s: %v", member.Name, err)
			continue
		}

		err = etcdClient.AddMember(ctx, nodeInfo.PeerUrls)
		etcdclient.LoggedClose(etcdClient)
		if err != nil {
			klog.Warningf("unable to add member %s on peer %s: %v", nodeInfo.PeerUrls, member.Name, err)
			continue
		}

		return member, nil
	}
	return nil, fmt.Errorf("unable to reach any cluster member, when trying to add new member %q", nodeInfo.PeerUrls)
}

func (s *etcdClusterState) etcdRemoveMember(ctx context.Context, member *etcdclient.EtcdProcessMember) error {
	for id, member := range s.members {
		etcdClient, err := s.newEtcdClient(member)
		if err != nil {
			klog.Warningf("unable to build client for member %s: %v", member.Name, err)
			continue
		}

		err = etcdClient.RemoveMember(ctx, member)
		etcdclient.LoggedClose(etcdClient)
		if err != nil {
			klog.Warningf("Remove member call failed on %s: %v", id, err)
			continue
		}
		return nil
	}
	return fmt.Errorf("unable to reach any cluster member, when trying to remove member %s", member)
}

func (s *etcdClusterState) etcdGet(ctx context.Context, key string) ([]byte, error) {
	for _, member := range s.members {
		if len(member.ClientURLs) == 0 {
			klog.Warningf("skipping member with no ClientURLs: %v", member)
			continue
		}

		etcdClient, err := s.newEtcdClient(member)
		if err != nil {
			klog.Warningf("unable to build client for member %s: %v", member, err)
			continue
		}

		response, err := etcdClient.Get(ctx, key, true)
		etcdclient.LoggedClose(etcdClient)
		if err != nil {
			klog.Warningf("error reading from member %s: %v", member, err)
			continue
		}

		return response, nil
	}

	return nil, fmt.Errorf("unable to reach any cluster member, when trying to read key %q", key)
}

func (s *etcdClusterState) etcdCreate(ctx context.Context, key string, value []byte) error {
	for _, member := range s.members {
		if len(member.ClientURLs) == 0 {
			klog.Warningf("skipping member with no ClientURLs: %v", member)
			continue
		}

		etcdClient, err := s.newEtcdClient(member)
		if err != nil {
			klog.Warningf("unable to build client for member %s: %v", member.Name, err)
			continue
		}

		err = etcdClient.Put(ctx, key, value)
		etcdclient.LoggedClose(etcdClient)
		if err != nil {
			return fmt.Errorf("error creating %q on member %s: %v", key, member, err)
		}

		return nil
	}

	return fmt.Errorf("unable to reach any cluster member, when trying to write key %q", key)
}
