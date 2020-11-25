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
	"encoding/json"
	"net/http"
	"net/url"

	etcd_client_v2 "go.etcd.io/etcd/client"
	"go.etcd.io/etcd/pkg/pathutil"
	"k8s.io/klog"
)

type v2SelfInfo struct {
	Name  string `json:"name"`
	ID    string `json:"id"`
	State string `json:"state"`
}

type statsSelfAction struct {
}

func (g *statsSelfAction) HTTPRequest(ep url.URL) *http.Request {
	u := ep
	u.Path = pathutil.CanonicalURLPath(u.Path + "/v2/stats/self")

	req, _ := http.NewRequest("GET", u.String(), nil)
	return req
}

func (c *V2Client) LocalNodeInfo(ctx context.Context) (*LocalNodeInfo, error) {
	act := &statsSelfAction{}
	resp, body, err := c.client.Do(ctx, act)
	if err != nil {
		return nil, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		if len(body) == 0 {
			return nil, etcd_client_v2.ErrEmptyBody
		}
		var vresp v2SelfInfo
		if err := json.Unmarshal(body, &vresp); err != nil {
			return nil, etcd_client_v2.ErrInvalidJSON
		}
		return &LocalNodeInfo{
			IsLeader: vresp.State == "StateLeader",
		}, nil
	default:
		var etcdErr etcd_client_v2.Error
		if err := json.Unmarshal(body, &etcdErr); err != nil {
			return nil, etcd_client_v2.ErrInvalidJSON
		}
		return nil, etcdErr
	}
}

func (c *V3Client) LocalNodeInfo(ctx context.Context) (*LocalNodeInfo, error) {
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
