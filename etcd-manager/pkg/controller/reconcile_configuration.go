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
	"reflect"
	"sort"

	"k8s.io/klog/v2"
	protoetcd "sigs.k8s.io/etcdadm/etcd-manager/pkg/apis/etcd"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/etcdclient"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/privateapi"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/urls"
)

func (m *EtcdController) updatePeerURLs(ctx context.Context, peerID privateapi.PeerId, p *etcdClusterPeerInfo, setPeerURLs []string) (bool, error) {
	clientURLs := p.info.NodeConfiguration.ClientUrls
	if p.info.EtcdState.Quarantined {
		clientURLs = p.info.NodeConfiguration.QuarantinedClientUrls
	}

	changed := false
	if len(setPeerURLs) == 0 {
		klog.Warningf("peer %q had no clientURLs, can't reconfigure peerUrls", peerID)
		return false, nil
	}

	klog.Infof("updating peer %q with peerURLs %v", peerID, setPeerURLs)

	etcdClient, err := etcdclient.NewClient(clientURLs, m.etcdClientTLSConfig)
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
	klog.Infof("reconfigured peer %v with peerUrls=%v", peerID, setPeerURLs)
	return changed, nil
}

func normalize(in []string) []string {
	var c []string
	c = append(c, in...)
	sort.Strings(c)
	return c
}

func (m *EtcdController) reconcileTLS(ctx context.Context, clusterState *etcdClusterState) (bool, error) {
	if m.disableEtcdTLS {
		klog.Infof("TLS configuration is disabled, won't enable TLS")
		return false, nil
	}

	for peerID, p := range clusterState.peers {
		if p.info == nil {
			continue
		}

		if p.info.NodeConfiguration == nil {
			klog.Warningf("peer %q did not have node configuration", peerID)
			continue
		}
		if p.info.EtcdState == nil {
			continue
		}
		if p.info.EtcdState.Cluster == nil {
			klog.Warningf("peer %q did not have etcd_state.cluster", peerID)
			continue
		}

		var node *protoetcd.EtcdNode
		for _, n := range p.info.EtcdState.Cluster.Nodes {
			if n.Name == p.info.NodeConfiguration.Name {
				node = n
			}
		}

		if node == nil {
			klog.Warningf("peer %q did not have node in etcd_state.cluster", peerID)
			continue
		}

		// We update the peerURLs first - that actually goes through raft and thus has more checks around it
		{
			expectedPeerURLs := p.info.NodeConfiguration.PeerUrls
			expectedPeerURLs = urls.RewriteScheme(expectedPeerURLs, "http://", "https://")

			member := clusterState.FindMember(peerID)
			if member == nil {
				klog.Warningf("peer %q was not part of cluster", peerID)
				continue
			}

			actualPeerURLs := member.PeerURLs

			expectedPeerURLs = normalize(expectedPeerURLs)
			actualPeerURLs = normalize(actualPeerURLs)

			if !reflect.DeepEqual(actualPeerURLs, expectedPeerURLs) {
				klog.Infof("peerURLs do not match: actual=%v, expected=%v", actualPeerURLs, expectedPeerURLs)

				c, err := m.updatePeerURLs(ctx, peerID, p, expectedPeerURLs)
				if c || err != nil {
					return c, err
				}
			}
		}

		if !node.TlsEnabled {
			request := &protoetcd.ReconfigureRequest{
				Header:      m.buildHeader(),
				Quarantined: p.info.EtcdState.Quarantined,
				EnableTls:   true,
			}

			klog.Infof("reconfiguring peer %q to enable TLS %v", peerID, request)

			response, err := p.peer.rpcReconfigure(ctx, request)
			if err != nil {
				return false, fmt.Errorf("error reconfiguring peer %v with %v: %v", peerID, request, err)
			}
			klog.Infof("reconfigured peer %v to enable TLS, response = %s", peerID, response)
			return true, nil
		}

	}
	return false, nil
}
