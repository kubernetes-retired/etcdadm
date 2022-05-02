/*
Copyright 2022 The Kubernetes Authors.

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

package hetzner

import (
	"context"
	"fmt"
	"strconv"

	"k8s.io/klog/v2"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/privateapi/discovery"
)

// HetznerVolumes also allows the discovery of peer nodes
var _ discovery.Interface = &HetznerVolumes{}

// Poll returns all etcd cluster peers.
func (a *HetznerVolumes) Poll() (map[string]discovery.Node, error) {
	peers := make(map[string]discovery.Node)

	klog.V(2).Infof("Discovering volumes for %q", a.nameTag)
	etcdVolumes, err := getMatchingVolumes(a.hcloudClient, a.matchTags)
	if err != nil {
		return nil, fmt.Errorf("failed to get matching volumes: %w", err)
	}

	for _, volume := range etcdVolumes {
		if volume.Server == nil {
			// Volume doesn't have a server attached yet
			continue
		}
		serverID := volume.Server.ID

		// TODO: Get all servers in a single requests to reduce the number of calls to the could API
		// Hetzner Cloud API is limited to 3600 requests/hour (see: https://docs.hetzner.cloud/#rate-limiting)
		server, _, err := a.hcloudClient.Server.GetByID(context.TODO(), serverID)
		if err != nil || server == nil {
			return nil, fmt.Errorf("failed to retrieve info for server %d: %w", serverID, err)
		}

		if len(server.PrivateNet) == 0 {
			return nil, fmt.Errorf("failed to find private net info for server %d: ", serverID)
		}
		serverPrivateIP := server.PrivateNet[0].IP

		klog.V(2).Infof("Discovered volume %s(%d) attached to server %s(%d)", volume.Name, volume.ID, server.Name, serverID)
		// We use the etcd node ID as the persistent identifier, because the data determines who we are
		node := discovery.Node{
			ID:        "vol-" + strconv.Itoa(volume.ID),
			Endpoints: []discovery.NodeEndpoint{{IP: serverPrivateIP.String()}},
		}
		peers[node.ID] = node
	}

	return peers, nil
}
