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

package etcdclient

import (
	"context"
	"k8s.io/klog/v2"
)

// LocalNodeInfo has information about the etcd member node we are connected to
type LocalNodeInfo struct {
	IsLeader bool
}

func (c *EtcdClient) LocalNodeInfo(ctx context.Context) (*LocalNodeInfo, error) {
	var lastErr error
	for _, endpoint := range c.endpoints {
		response, err := c.client.Status(ctx, endpoint)
		if err != nil {
			klog.Warningf("unable to get status from %q: %v", endpoint, err)
			lastErr = err
		} else {
			return &LocalNodeInfo{
				IsLeader: response.Header.MemberId == response.Leader,
			}, nil
		}
	}
	return nil, lastErr
}
