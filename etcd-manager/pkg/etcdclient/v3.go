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
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	etcd_client_v3 "go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/version"
	"k8s.io/klog"
)

type V3Client struct {
	endpoints   []string
	client      *etcd_client_v3.Client
	kv          etcd_client_v3.KV
	cluster     etcd_client_v3.Cluster
	maintenance etcd_client_v3.Maintenance
	tlsConfig   *tls.Config
}

var _ EtcdClient = &V3Client{}

func NewV3Client(endpoints []string, tlsConfig *tls.Config) (EtcdClient, error) {
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("no endpoints provided")
	}
	cfg := etcd_client_v3.Config{
		Endpoints:            endpoints,
		DialTimeout:          5 * time.Second,
		TLS:                  tlsConfig,
		DialKeepAliveTime:    2 * time.Second,
		DialKeepAliveTimeout: 2 * time.Second,
	}
	etcdClient, err := etcd_client_v3.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("error building etcd client for %s: %v", endpoints[0], err)
	}

	kv := etcd_client_v3.NewKV(etcdClient)
	maintenance := etcd_client_v3.NewMaintenance(etcdClient)
	return &V3Client{
		endpoints:   endpoints,
		client:      etcdClient,
		kv:          kv,
		maintenance: maintenance,
		cluster:     etcd_client_v3.NewCluster(etcdClient),
		tlsConfig:   tlsConfig,
	}, nil
}

func (c *V3Client) Close() error {
	return c.client.Close()
}

func (c *V3Client) String() string {
	return "V3Client:[" + strings.Join(c.endpoints, ",") + "]"
}

// ServerVersion returns the version of etcd running
func (c *V3Client) ServerVersion(ctx context.Context) (string, error) {
	tr := &http.Transport{
		TLSClientConfig: c.tlsConfig,
	}
	httpClient := &http.Client{Transport: tr}

	for _, endpoint := range c.endpoints {
		u := endpoint
		if !strings.HasSuffix(u, "/") {
			u += "/"
		}
		u += "version"

		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			klog.Warningf("failed to fetch %s: %v", u, err)
			continue
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			klog.Warningf("failed to fetch %s: %v", u, err)
			continue
		}
		body, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			klog.Warningf("failed to read %s: %v", u, err)
			continue
		}

		v := &version.Versions{}
		if err := json.Unmarshal(body, v); err != nil {
			klog.Warningf("failed to parse %s %s: %v", u, string(body), err)
			continue
		}

		return v.Server, nil
	}
	return "", fmt.Errorf("could not fetch server version")
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
	klog.V(4).Infof("put %s response %v", key, response)
	return nil
}

func (c *V3Client) CopyTo(ctx context.Context, dest NodeSink) (int, error) {
	count := 0

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
			return count, err
		}
		gotMore := false
		for _, kv := range response.Kvs {
			key := string(kv.Key)
			if key == lastKey {
				continue
			}
			gotMore = true
			klog.Infof("copying key %q", key)
			if err := dest.Put(ctx, key, kv.Value); err != nil {
				return count, fmt.Errorf("error writing key %q to destination: %v", key, err)
			}
			count++
			lastKey = key
		}

		if !gotMore {
			break
		}
	}

	return count, nil
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

func (c *V3Client) LeaderID(ctx context.Context) (string, error) {
	response, err := c.maintenance.Status(ctx, c.endpoints[0])
	if err != nil {
		return "", err
	}
	leaderID := response.Leader
	if leaderID == 0 {
		return "", nil
	}

	return strconv.FormatUint(leaderID, 10), nil
}

func (c *V3Client) AddMember(ctx context.Context, peerURLs []string) error {
	_, err := c.cluster.MemberAdd(ctx, peerURLs)
	return err
}

func (c *V3Client) SetPeerURLs(ctx context.Context, member *EtcdProcessMember, peerURLs []string) error {
	_, err := c.cluster.MemberUpdate(ctx, member.idv3, peerURLs)
	return err
}

func (c *V3Client) RemoveMember(ctx context.Context, member *EtcdProcessMember) error {
	_, err := c.cluster.MemberRemove(ctx, member.idv3)
	return err
}

func (c *V3Client) SnapshotSave(ctx context.Context, path string) error {
	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("error creating snapshot file: %v", err)
	}
	defer out.Close()

	in, err := c.client.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("error making snapshot: %v", err)
	}
	defer in.Close()

	gz := gzip.NewWriter(out)
	if _, err := io.Copy(gz, in); err != nil {
		return fmt.Errorf("error copying snapshot: %v", err)
	}

	if err := gz.Close(); err != nil {
		return fmt.Errorf("error copying snapshot: %v", err)
	}

	return nil
}

func (c *V3Client) SupportsSnapshot() bool {
	return true
}
