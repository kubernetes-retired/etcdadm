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

package alicloud

import (
	"fmt"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"k8s.io/klog/v2"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/privateapi/discovery"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/volumes"
)

// AlicloudVolumes allows us to discover our peer nodes
var _ discovery.Interface = &AlicloudVolumes{}

func (a *AlicloudVolumes) Poll() (map[string]discovery.Node, error) {
	nodes := make(map[string]discovery.Node)

	allVolumes, err := a.findVolumes(false)
	if err != nil {
		return nil, err
	}

	instanceToVolumeMap := make(map[string]*volumes.Volume)
	for _, v := range allVolumes {
		if v.AttachedTo != "" {
			instanceToVolumeMap[v.AttachedTo] = v
		}
	}

	if len(instanceToVolumeMap) != 0 {
		request := ecs.CreateDescribeInstancesRequest()
		request.RegionId = a.region
		request.PageSize = requests.NewInteger(PageSizeLarge)

		var ids []string
		for id := range instanceToVolumeMap {
			ids = append(ids, id)
		}
		request.InstanceIds = arrayString(ids)

		response, err := a.ecs.DescribeInstances(request)
		if err != nil {
			return nil, fmt.Errorf("error from Alicloud DescribeInstances: %v", err)
		}

		for _, instance := range response.Instances.Instance {
			volume := instanceToVolumeMap[instance.InstanceId]
			if volume == nil {
				// unexpected ... we constructed the request from the map!
				klog.Errorf("instance not found: %q", instance.InstanceId)
				continue
			}

			// We use the etcd node ID as the persistent identifier, because the data determines who we are
			node := discovery.Node{
				ID: volume.EtcdName,
			}
			privateIPs := instance.InnerIpAddress.IpAddress
			if len(privateIPs) != 0 {
				ip := privateIPs[0]
				node.Endpoints = append(node.Endpoints, discovery.NodeEndpoint{IP: ip})
			}
			for _, ni := range instance.NetworkInterfaces.NetworkInterface {
				if ni.PrimaryIpAddress != "" {
					ip := ni.PrimaryIpAddress
					node.Endpoints = append(node.Endpoints, discovery.NodeEndpoint{IP: ip})
				}
			}
			nodes[node.ID] = node
		}
	}

	return nodes, nil
}
