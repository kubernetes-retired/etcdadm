package controller

import (
	"context"
	"fmt"
	"reflect"
	"sort"

	"github.com/golang/glog"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/pkg/etcdclient"
	"kope.io/etcd-manager/pkg/privateapi"
	"kope.io/etcd-manager/pkg/urls"
)

func (m *EtcdController) updatePeerURLs(ctx context.Context, peerID privateapi.PeerId, p *etcdClusterPeerInfo, setPeerURLs []string) (bool, error) {
	clientURLs := p.info.NodeConfiguration.ClientUrls
	if p.info.EtcdState.Quarantined {
		clientURLs = p.info.NodeConfiguration.QuarantinedClientUrls
	}

	changed := false
	if len(setPeerURLs) == 0 {
		glog.Warningf("peer %q had no clientURLs, can't reconfigure peerUrls", peerID)
		return false, nil
	}

	glog.Infof("updating peer %q with peerURLs %v", peerID, setPeerURLs)

	etcdClient, err := etcdclient.NewClient(p.info.EtcdState.EtcdVersion, clientURLs, m.etcdClientTLSConfig)
	if err != nil {
		return changed, fmt.Errorf("unable to reach peer %s: %v", peerID, err)
	}

	defer etcdclient.LoggedClose(etcdClient)

	members, err := etcdClient.ListMembers(ctx)
	if err != nil {
		return changed, fmt.Errorf("error listing members on peer %q: %v", peerID, err)
	}

	var member *etcdclient.EtcdProcessMember
	for _, m := range members {
		if m.Name == string(peerID) {
			member = m
		}
	}

	if member == nil {
		return changed, fmt.Errorf("peer %q had no member in cluster, can't reconfigure peerUrls", peerID)
	}

	if err := etcdClient.SetPeerURLs(ctx, member, setPeerURLs); err != nil {
		return changed, fmt.Errorf("error reconfiguring peer %v with peerUrls=%v: %v", peerID, setPeerURLs, err)
	}

	changed = true
	glog.Infof("reconfigured peer %v with peerUrls=%v", peerID, setPeerURLs)
	return changed, nil
}

func normalize(in []string) []string {
	var c []string
	for _, s := range in {
		c = append(c, s)
	}
	sort.Strings(c)
	return c
}

func (m *EtcdController) reconcileTLS(ctx context.Context, clusterState *etcdClusterState) (bool, error) {
	if m.disableEtcdTLS {
		glog.Infof("TLS configuration is disabled, won't enable TLS")
		return false, nil
	}

	for peerID, p := range clusterState.peers {
		if p.info == nil {
			continue
		}

		if p.info.NodeConfiguration == nil {
			glog.Warningf("peer %q did not have node configuration", peerID)
			continue
		}
		if p.info.EtcdState == nil {
			continue
		}
		if p.info.EtcdState.Cluster == nil {
			glog.Warningf("peer %q did not have etcd_state.cluster", peerID)
			continue
		}

		var node *protoetcd.EtcdNode
		for _, n := range p.info.EtcdState.Cluster.Nodes {
			if n.Name == p.info.NodeConfiguration.Name {
				node = n
			}
		}

		if node == nil {
			glog.Warningf("peer %q did not have node in etcd_state.cluster", peerID)
			continue
		}

		// We update the peerURLs first - that actually goes through raft and thus has more checks around it
		{
			expectedPeerURLs := p.info.NodeConfiguration.PeerUrls
			expectedPeerURLs = urls.RewriteScheme(expectedPeerURLs, "http://", "https://")

			member := clusterState.FindMember(peerID)
			if member == nil {
				glog.Warningf("peer %q was not part of cluster", peerID)
				continue
			}

			actualPeerURLs := member.PeerURLs

			expectedPeerURLs = normalize(expectedPeerURLs)
			actualPeerURLs = normalize(actualPeerURLs)

			if !reflect.DeepEqual(actualPeerURLs, expectedPeerURLs) {
				glog.Infof("peerURLs do not match: actual=%v, expected=%v", actualPeerURLs, expectedPeerURLs)

				c, err := m.updatePeerURLs(ctx, peerID, p, expectedPeerURLs)
				if c || err != nil {
					return c, err
				}
			}
		}

		if node.TlsEnabled == false {
			request := &protoetcd.ReconfigureRequest{
				Header:      m.buildHeader(),
				Quarantined: p.info.EtcdState.Quarantined,
				EnableTls:   true,
			}

			glog.Infof("reconfiguring peer %q to enable TLS %v", peerID, request)

			response, err := p.peer.rpcReconfigure(ctx, request)
			if err != nil {
				return false, fmt.Errorf("error reconfiguring peer %v with %v: %v", peerID, request, err)
			}
			glog.Infof("reconfigured peer %v to enable TLS, response = %s", peerID, response)
			return true, nil
		}

	}
	return false, nil
}
