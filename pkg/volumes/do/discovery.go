/*
Copyright 2018 The Kubernetes Authors.

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

package do

import (
	"fmt"

	"github.com/golang/glog"
	"kope.io/etcd-manager/pkg/privateapi/discovery"
	"kope.io/etcd-manager/pkg/volumes"
)

// DO Volumes also allows us to discover our peer nodes
var _ discovery.Interface = &DOVolumes{}

func (a *DOVolumes) Poll() (map[string]discovery.Node, error) {
	nodes := make(map[string]discovery.Node)

	glog.V(2).Infof("Polling all DO Volumes")

	allVolumes, err := a.findVolumes(false)
	if err != nil {
		return nil, fmt.Errorf("failed to discover any volumes: %s", err)
	}

	instanceToVolumeMap := make(map[string]*volumes.Volume)
	for _, v := range allVolumes {
		if v.AttachedTo != "" {
			instanceToVolumeMap[v.AttachedTo] = v
		}
	}

	for _, volume := range instanceToVolumeMap {
		var myIP, _ = a.MyIP()

		glog.V(2).Infof("Discovered a matching DO Volume for this instance with instanceid=%s volumename=%s nameTag=%s myIP=%s etcdName=%s", volume.AttachedTo, volume.ProviderID, a.nameTag, myIP, volume.EtcdName)

		// We use the etcd node ID as the persistent identifier, because the data determines who we are
		node := discovery.Node{
			ID: volume.EtcdName,
		}

		node.Endpoints = append(node.Endpoints, discovery.NodeEndpoint{IP: myIP})
		nodes[node.ID] = node
	}

	return nodes, nil
}
