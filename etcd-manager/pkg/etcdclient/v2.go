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
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	etcd_client_v2 "go.etcd.io/etcd/client"
	"k8s.io/klog"
)

// V2Client is a client for the etcd v2 API, implementing EtcdClient
type V2Client struct {
	clientUrls []string
	client     etcd_client_v2.Client
	keys       etcd_client_v2.KeysAPI
	members    etcd_client_v2.MembersAPI
}

var _ EtcdClient = &V2Client{}

func NewV2Client(clientUrls []string, tlsConfig *tls.Config) (EtcdClient, error) {
	if len(clientUrls) == 0 {
		return nil, fmt.Errorf("no endpoints provided")
	}

	// Copy of etcd_client_v2.DefaultTransport, it's not safe to copy http.Transport
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     tlsConfig,
	}

	cfg := etcd_client_v2.Config{
		Endpoints:               clientUrls,
		Transport:               transport,
		HeaderTimeoutPerRequest: 10 * time.Second,
	}

	etcdClient, err := etcd_client_v2.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("error building etcd client for %s: %v", clientUrls, err)
	}

	keysAPI := etcd_client_v2.NewKeysAPI(etcdClient)

	return &V2Client{
		clientUrls: clientUrls,
		client:     etcdClient,
		keys:       keysAPI,
		members:    etcd_client_v2.NewMembersAPI(etcdClient),
	}, nil
}

func (c *V2Client) String() string {
	return "V2Client:[" + strings.Join(c.clientUrls, ",") + "]"
}

func (c *V2Client) Close() error {
	// Nothing to close
	return nil
}

// ServerVersion returns the version of etcd running
func (c *V2Client) ServerVersion(ctx context.Context) (string, error) {
	v, err := c.client.GetVersion(ctx)
	if err != nil {
		return "", err
	}
	return v.Server, nil
}

func (c *V2Client) Get(ctx context.Context, key string, quorum bool) ([]byte, error) {
	r, err := c.keys.Get(ctx, key, &etcd_client_v2.GetOptions{Quorum: quorum})
	if err != nil {
		if etcdError, ok := err.(etcd_client_v2.Error); ok {
			if etcdError.Code == etcd_client_v2.ErrorCodeKeyNotFound {
				return nil, nil
			}
		}

		return nil, err
	}
	return []byte(r.Node.Value), nil
}

func (c *V2Client) Create(ctx context.Context, key string, value []byte) error {
	options := &etcd_client_v2.SetOptions{}
	options.PrevExist = etcd_client_v2.PrevNoExist
	_, err := c.keys.Set(ctx, key, string(value), options)
	if err != nil {
		return err
	}
	return nil
}

func (c *V2Client) Put(ctx context.Context, key string, value []byte) error {
	options := &etcd_client_v2.SetOptions{}
	_, err := c.keys.Set(ctx, key, string(value), options)
	if err != nil {
		return err
	}
	return nil
}

func (c *V2Client) ListMembers(ctx context.Context) ([]*EtcdProcessMember, error) {
	response, err := c.members.List(ctx)
	if err != nil {
		return nil, err
	}
	var members []*EtcdProcessMember
	for _, m := range response {
		members = append(members, &EtcdProcessMember{
			ClientURLs:  m.ClientURLs,
			PeerURLs:    m.PeerURLs,
			ID:          m.ID,
			idv2:        m.ID,
			Name:        m.Name,
			etcdVersion: "2.x",
		})
	}
	return members, nil
}

func (c *V2Client) LeaderID(ctx context.Context) (string, error) {
	leader, err := c.members.Leader(ctx)
	if err != nil {
		return "", err
	}
	if leader == nil {
		return "", nil
	}
	return leader.ID, nil
}

func (c *V2Client) AddMember(ctx context.Context, peerURLs []string) error {
	if len(peerURLs) == 0 {
		return fmt.Errorf("AddMember with empty peerURLs")
	}
	if len(peerURLs) != 1 {
		return fmt.Errorf("etcd V2 API does not support add with multiple peer urls: %v", peerURLs)
	}
	_, err := c.members.Add(ctx, peerURLs[0])
	return err
}

func (c *V2Client) SetPeerURLs(ctx context.Context, member *EtcdProcessMember, peerURLs []string) error {
	err := c.members.Update(ctx, member.idv2, peerURLs)
	return err
}

func (c *V2Client) RemoveMember(ctx context.Context, member *EtcdProcessMember) error {
	err := c.members.Remove(ctx, member.idv2)
	return err
}

func (c *V2Client) CopyTo(ctx context.Context, dest NodeSink) (int, error) {
	return c.copySubtree(ctx, "/", dest)
}

func (c *V2Client) copySubtree(ctx context.Context, p string, dest NodeSink) (int, error) {
	count := 0
	opts := &etcd_client_v2.GetOptions{
		Quorum: false,
		// We don't do Recursive: true, to avoid huge responses
	}
	klog.V(4).Infof("listing keys under %s", p)
	response, err := c.keys.Get(ctx, p, opts)
	if err != nil {
		return count, fmt.Errorf("error reading %q: %v", p, err)
	}

	if response.Node == nil {
		return count, fmt.Errorf("node %q not found", p)
	}

	if !response.Node.Dir {
		if err := dest.Put(ctx, response.Node.Key, []byte(response.Node.Value)); err != nil {
			return count, fmt.Errorf("error writing node: %v", err)
		}
		count++
	}

	for _, n := range response.Node.Nodes {
		subCount, err := c.copySubtree(ctx, n.Key, dest)
		if err != nil {
			return count, err
		}
		count += subCount
	}

	return count, nil
}

func (c *V2Client) SnapshotSave(ctx context.Context, path string) error {
	return fmt.Errorf("SnapshotSave is not supported in V2")
}

func (c *V2Client) SupportsSnapshot() bool {
	return false
}
