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

package gce

import (
	"k8s.io/klog"
	"kope.io/etcd-manager/pkg/privateapi/discovery"
	"kope.io/etcd-manager/pkg/volumes"
)

// GCEVolumes also allows us to discover our peer nodes
var _ discovery.Interface = &GCEVolumes{}

func (g *GCEVolumes) Poll() (map[string]discovery.Node, error) {
	nodes := make(map[string]discovery.Node)

	allVolumes, err := g.findVolumes(false)
	if err != nil {
		return nil, err
	}

	instanceToVolumeMap := make(map[string]*volumes.Volume)
	for _, v := range allVolumes {
		if v.AttachedTo != "" {
			instanceToVolumeMap[v.AttachedTo] = v
		}
	}

	for i, volume := range instanceToVolumeMap {
		u, err := ParseGoogleCloudURL(i)
		if err != nil {
			klog.Warningf("cannot parse instance url %q: %v", i, err)
			continue
		}

		if u.Project == "" || u.Zone == "" || u.Name == "" {
			klog.Warningf("unexpected format for isntance url %q", i)
			continue
		}

		instance, err := g.compute.Instances.Get(u.Project, u.Zone, u.Name).Do()
		if err != nil {
			klog.Warningf("error getting instance %s/%s/%s: %v", u.Project, u.Zone, u.Name, err)
			continue
		}

		// We use the etcd node ID as the persistent identifier, because the data determines who we are
		node := discovery.Node{
			ID: volume.EtcdName,
		}
		for _, ni := range instance.NetworkInterfaces {
			// TODO: Check e.g. Network

			if ni.NetworkIP != "" {
				ip := ni.NetworkIP
				node.Endpoints = append(node.Endpoints, discovery.NodeEndpoint{IP: ip})
			}
		}

		nodes[node.ID] = node
	}

	return nodes, nil
}
