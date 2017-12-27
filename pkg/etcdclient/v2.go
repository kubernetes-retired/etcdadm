package etcdclient

import (
	"context"
	"fmt"
	"time"

	etcd_client_v2 "github.com/coreos/etcd/client"
	"github.com/golang/glog"
)

type V2Client struct {
	keys    etcd_client_v2.KeysAPI
	members etcd_client_v2.MembersAPI
}

func NewV2Client(clientUrls []string) (EtcdClient, error) {
	if len(clientUrls) == 0 {
		return nil, fmt.Errorf("no clientURLs provided")
	}
	cfg := etcd_client_v2.Config{
		Endpoints:               clientUrls,
		Transport:               etcd_client_v2.DefaultTransport,
		HeaderTimeoutPerRequest: 10 * time.Second,
	}
	etcdClient, err := etcd_client_v2.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("error building etcd client for %s: %v", clientUrls, err)
	}

	keysAPI := etcd_client_v2.NewKeysAPI(etcdClient)

	return &V2Client{
		keys:    keysAPI,
		members: etcd_client_v2.NewMembersAPI(etcdClient),
	}, nil
}

func (c *V2Client) Get(ctx context.Context, key string, quorum bool) ([]byte, error) {
	r, err := c.keys.Get(ctx, key, &etcd_client_v2.GetOptions{Quorum: quorum})
	if err != nil {
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

func (c *V2Client) RemoveMember(ctx context.Context, member *EtcdProcessMember) error {
	err := c.members.Remove(ctx, member.idv2)
	return err
}

func (c *V2Client) CopyTo(ctx context.Context, dest EtcdClient) error {
	return c.copySubtree(ctx, "/", dest)
}

func (c *V2Client) copySubtree(ctx context.Context, p string, dest EtcdClient) error {
	opts := &etcd_client_v2.GetOptions{
		Quorum: false,
		// We don't do Recursive: true, to avoid huge responses
	}
	glog.V(4).Infof("listing keys under %s", p)
	response, err := c.keys.Get(ctx, p, opts)
	if err != nil {
		return fmt.Errorf("error reading %q: %v", p, err)
	}

	if err != nil {
		return fmt.Errorf("error reading %q: %v", string(p), err)
	}

	if response.Node == nil {
		return fmt.Errorf("node %q not found", p)
	}

	if !response.Node.Dir {
		if err := dest.Put(ctx, response.Node.Key, []byte(response.Node.Value)); err != nil {
			return fmt.Errorf("error writing node: %v", err)
		}
	}

	for _, n := range response.Node.Nodes {
		err := c.copySubtree(ctx, n.Key, dest)
		if err != nil {
			return err
		}
	}

	return nil
}
