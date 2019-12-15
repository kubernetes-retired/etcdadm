package etcdclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	etcd_client_v2 "github.com/coreos/etcd/client"
	"github.com/coreos/etcd/pkg/pathutil"
	"github.com/golang/glog"
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
			glog.Warningf("unable to get status from %q: %v", endpoint, err)
			lastErr = err
		} else {
			return &LocalNodeInfo{
				IsLeader: response.Header.MemberId == response.Leader,
			}, nil
		}
	}
	return nil, lastErr
}
