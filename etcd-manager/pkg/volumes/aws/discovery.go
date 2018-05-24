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

package aws

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"

	"kope.io/etcd-manager/pkg/privateapi/discovery"
	"kope.io/etcd-manager/pkg/volumes"
)

// AWSVolumes also allows us to discover our peer nodes
var _ discovery.Interface = &AWSVolumes{}

func (a *AWSVolumes) Poll() (map[string]discovery.Node, error) {
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
		request := &ec2.DescribeInstancesInput{}
		for id := range instanceToVolumeMap {
			request.InstanceIds = append(request.InstanceIds, aws.String(id))
		}

		response, err := a.ec2.DescribeInstances(request)
		if err != nil {
			return nil, fmt.Errorf("error from AWS DescribeInstances: %v", err)
		}

		for _, reservation := range response.Reservations {
			for _, instance := range reservation.Instances {
				volume := instanceToVolumeMap[aws.StringValue(instance.InstanceId)]
				if volume == nil {
					// unexpected ... we constructed the request from the map!
					glog.Errorf("instance not found: %q", aws.StringValue(instance.InstanceId))
					continue
				}

				// We use the etcd node ID as the persistent identifier, because the data determines who we are
				node := discovery.Node{
					ID: volume.EtcdName,
				}
				if aws.StringValue(instance.PrivateIpAddress) != "" {
					e := fmt.Sprintf(a.endpointFormat, aws.StringValue(instance.PrivateIpAddress))
					node.Endpoints = append(node.Endpoints, discovery.NodeEndpoint{Endpoint: e})
				}
				for _, ni := range instance.NetworkInterfaces {
					if aws.StringValue(ni.PrivateIpAddress) != "" {
						e := fmt.Sprintf(a.endpointFormat, aws.StringValue(ni.PrivateIpAddress))
						node.Endpoints = append(node.Endpoints, discovery.NodeEndpoint{Endpoint: e})
					}
				}
				nodes[node.ID] = node
			}
		}
	}

	return nodes, nil
}
