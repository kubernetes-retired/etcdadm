package controller

import (
	"bytes"
	"context"
	"fmt"

	"github.com/golang/glog"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/pkg/etcdclient"
	"kope.io/etcd-manager/pkg/privateapi"
)

type EtcdMemberId string

type etcdClusterState struct {
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
	return fmt.Sprintf("etcdClusterPeerInfo{peer=%s, etcState=%s}", p.peer, p.info)
}

func (s *etcdClusterState) etcdAddMember(ctx context.Context, nodeInfo *protoetcd.EtcdNode) (*etcdclient.EtcdProcessMember, error) {
	for _, member := range s.members {
		etcdClient, err := member.NewClient()
		if err != nil {
			glog.Warningf("unable to build client for member %s: %v", member.Name, err)
			continue
		}

		err = etcdClient.AddMember(ctx, nodeInfo.PeerUrls)
		etcdClient.Close()
		if err != nil {
			glog.Warningf("unable to add member %s on peer %s: %v", nodeInfo.PeerUrls, member.Name, err)
			continue
		}

		return member, nil
	}
	return nil, fmt.Errorf("unable to reach any cluster member, when trying to add new member %q", nodeInfo.PeerUrls)
}

func (s *etcdClusterState) etcdRemoveMember(ctx context.Context, member *etcdclient.EtcdProcessMember) error {
	for id, member := range s.members {
		etcdClient, err := member.NewClient()
		if err != nil {
			glog.Warningf("unable to build client for member %s: %v", member.Name, err)
			continue
		}

		err = etcdClient.RemoveMember(ctx, member)
		etcdClient.Close()
		if err != nil {
			glog.Warningf("Remove member call failed on %s: %v", id, err)
			continue
		}
		return nil
	}
	return fmt.Errorf("unable to reach any cluster member, when trying to remove member %", member)
}

func (s *etcdClusterState) etcdGet(ctx context.Context, key string) ([]byte, error) {
	for _, member := range s.members {
		if len(member.ClientURLs) == 0 {
			glog.Warningf("skipping member with no ClientURLs: %v", member)
			continue
		}

		etcdClient, err := member.NewClient()
		if err != nil {
			glog.Warningf("unable to build client for member %s: %v", member, err)
			continue
		}

		response, err := etcdClient.Get(ctx, key, true)
		etcdClient.Close()
		if err != nil {
			glog.Warningf("error reading from member %s: %v", member, err)
			continue
		}

		return response, nil
	}

	return nil, fmt.Errorf("unable to reach any cluster member, when trying to read key %q", key)
}

func (s *etcdClusterState) etcdCreate(ctx context.Context, key string, value []byte) error {
	for _, member := range s.members {
		if len(member.ClientURLs) == 0 {
			glog.Warningf("skipping member with no ClientURLs: %v", member)
			continue
		}

		etcdClient, err := member.NewClient()
		if err != nil {
			glog.Warningf("unable to build client for member %s: %v", member.Name, err)
			continue
		}

		err = etcdClient.Put(ctx, key, value)
		etcdClient.Close()
		if err != nil {
			return fmt.Errorf("error creating %q on member %s: %v", key, member, err)
		}

		return nil
	}

	return fmt.Errorf("unable to reach any cluster member, when trying to write key %q", key)
}
