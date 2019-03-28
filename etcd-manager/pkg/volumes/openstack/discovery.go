/*
Copyright 2019 The Kubernetes Authors.

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

package openstack

import (
	"github.com/golang/glog"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"kope.io/etcd-manager/pkg/privateapi/discovery"
	"kope.io/etcd-manager/pkg/volumes"
)

// OpenstackVolumes also allows us to discover our peer nodes
var _ discovery.Interface = &OpenstackVolumes{}

func (os *OpenstackVolumes) Poll() (map[string]discovery.Node, error) {

	allVolumes, err := os.FindVolumes()
	if err != nil {
		return nil, err
	}

	nodes := make(map[string]discovery.Node)
	instanceToVolumeMap := make(map[string]*volumes.Volume)
	for _, v := range allVolumes {
		if v.AttachedTo != "" {
			instanceToVolumeMap[v.AttachedTo] = v
		}
	}

	for i, volume := range instanceToVolumeMap {
		server, err := servers.Get(os.computeClient, i).Extract()
		if err != nil {
			glog.Warningf("Could not find server %s: %v", server.Name, err)
			continue
		}

		// We use the etcd node ID as the persistent identifier, because the data determines who we are
		node := discovery.Node{
			ID: volume.EtcdName,
		}
		address, err := GetServerFixedIP(server)
		if err != nil {
			glog.Warningf("Could not find servers fixed ip %s: %v", server.Name, err)
			continue
		}
		node.Endpoints = append(node.Endpoints, discovery.NodeEndpoint{IP: address})
		nodes[node.ID] = node
	}

	return nodes, nil
}
