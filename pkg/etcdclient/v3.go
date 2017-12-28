package etcdclient

import (
	"context"
	"fmt"
	"strconv"
	"time"

	etcd_client_v3 "github.com/coreos/etcd/clientv3"
	"github.com/golang/glog"
)

type V3Client struct {
	kv      etcd_client_v3.KV
	cluster etcd_client_v3.Cluster
}

func NewV3Client(clientUrls []string) (EtcdClient, error) {
	cfg := etcd_client_v3.Config{
		Endpoints:   clientUrls,
		DialTimeout: 10 * time.Second,
	}
	etcdClient, err := etcd_client_v3.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("error building etcd client for %s: %v", clientUrls[0], err)
	}

	kv := etcd_client_v3.NewKV(etcdClient)

	return &V3Client{
		kv:      kv,
		cluster: etcd_client_v3.NewCluster(etcdClient),
	}, nil
}

func (c *V3Client) Get(ctx context.Context, key string, quorum bool) ([]byte, error) {
	var opts []etcd_client_v3.OpOption
	if quorum {
		// Quorum is the default in etcd3
		// TODO: Is this right?
		//opts = append(opts, etcd_client_v3.WithQuorum())
	}
	r, err := c.kv.Get(ctx, key, opts...)
	if err != nil {
		return nil, err
	}
	if len(r.Kvs) == 0 {
		return nil, nil
	}
	return r.Kvs[0].Value, nil
}

func (c *V3Client) Create(ctx context.Context, key string, value []byte) error {
	txn := c.kv.Txn(ctx)
	txn.If(etcd_client_v3.Compare(etcd_client_v3.CreateRevision(key), "=", 0))
	txn.Then(etcd_client_v3.OpPut(key, string(value)))
	response, err := txn.Commit()
	if err != nil {
		return err
	}
	if !response.Succeeded {
		return fmt.Errorf("key %q already exists", key)
	}
	return nil
}

func (c *V3Client) Put(ctx context.Context, key string, value []byte) error {
	response, err := c.kv.Put(ctx, key, string(value))
	if err != nil {
		return err
	}
	glog.V(4).Infof("put %s response %v", key, response)
	return nil
}

func (c *V3Client) CopyTo(ctx context.Context, dest EtcdClient) error {
	limit := etcd_client_v3.WithLimit(1000)
	sort := etcd_client_v3.WithSort(etcd_client_v3.SortByKey, etcd_client_v3.SortAscend)

	var lastKey string
	for {
		etcdFrom := lastKey
		if etcdFrom == "" {
			etcdFrom = "\x00"
		}
		response, err := c.kv.Get(ctx, etcdFrom, etcd_client_v3.WithFromKey(), sort, limit)
		if err != nil {
			return err
		}
		gotMore := false
		for _, kv := range response.Kvs {
			key := string(kv.Key)
			if key == lastKey {
				continue
			}
			gotMore = true
			glog.Infof("copying key %q", key)
			if err := dest.Put(ctx, key, kv.Value); err != nil {
				return fmt.Errorf("error writing key %q to destination: %v", key, err)
			}
			lastKey = key
		}

		if !gotMore {
			break
		}
	}
	return nil
}

func (c *V3Client) ListMembers(ctx context.Context) ([]*EtcdProcessMember, error) {
	response, err := c.cluster.MemberList(ctx)
	if err != nil {
		return nil, err
	}
	var members []*EtcdProcessMember
	for _, m := range response.Members {
		members = append(members, &EtcdProcessMember{
			ClientURLs:  m.ClientURLs,
			PeerURLs:    m.PeerURLs,
			ID:          strconv.FormatUint(m.ID, 10),
			idv3:        m.ID,
			Name:        m.Name,
			etcdVersion: "3.x",
		})
	}
	return members, nil
}

func (c *V3Client) AddMember(ctx context.Context, peerURLs []string) error {
	_, err := c.cluster.MemberAdd(ctx, peerURLs)
	return err
}

func (c *V3Client) RemoveMember(ctx context.Context, member *EtcdProcessMember) error {
	_, err := c.cluster.MemberRemove(ctx, member.idv3)
	return err
}
