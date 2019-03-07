/**
 *   Copyright 2018 The etcdadm authors
 *
 *   Licensed under the Apache License, Version 2.0 (the "License");
 *   you may not use this file except in compliance with the License.
 *   You may obtain a copy of the License at
 *
 *       http://www.apache.org/licenses/LICENSE-2.0
 *
 *   Unless required by applicable law or agreed to in writing, software
 *   distributed under the License is distributed on an "AS IS" BASIS,
 *   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *   See the License for the specific language governing permissions and
 *   limitations under the License.
 */

package etcd

import (
	"fmt"
	"strings"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/coreos/etcd/snapshot"

	"sigs.k8s.io/etcdadm/apis"
)

// ClientForEndpoint returns an etcd client that will use the given etcd endpoint.
func ClientForEndpoint(endpoint string, cfg *apis.EtcdAdmConfig) (*clientv3.Client, error) {
	tlsInfo := transport.TLSInfo{
		CertFile:      cfg.EtcdctlCertFile,
		KeyFile:       cfg.EtcdctlKeyFile,
		TrustedCAFile: cfg.TrustedCAFile,
	}
	tlsConfig, err := tlsInfo.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to create TLS client: %v", err)
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{endpoint},
		DialTimeout: 5 * time.Second,
		TLS:         tlsConfig,
	})
	return cli, err
}

// MemberForPeerURLs searches the list for a member with matching peerURLs.
func MemberForPeerURLs(members []*etcdserverpb.Member, peerURLs []string) (*etcdserverpb.Member, bool) {
	for _, m := range members {
		if stringSlicesEqual(m.PeerURLs, peerURLs) {
			return m, true
		}
	}
	return nil, false
}

// stringSlicesEqual compares two string slices for equality
func stringSlicesEqual(l, r []string) bool {
	if len(l) != len(r) {
		return false
	}
	for i := range l {
		if l[i] != r[i] {
			return false
		}
	}
	return true
}

// MemberForID searches the list for a member with a matching ID.
func MemberForID(members []*etcdserverpb.Member, id uint64) (*etcdserverpb.Member, bool) {
	for _, m := range members {
		if m.ID == id {
			return m, true
		}
	}
	return nil, false
}

// Started checks whether the member has started.
func Started(member *etcdserverpb.Member) bool {
	unstarted := (member.Name == "" && len(member.ClientURLs) == 0)
	return !unstarted
}

// RestoreSnapshot initializes the etcd data directory from a snapshot
func RestoreSnapshot(cfg *apis.EtcdAdmConfig) error {
	sp := snapshot.NewV3(nil)

	return sp.Restore(snapshot.RestoreConfig{
		SnapshotPath:        cfg.Snapshot,
		Name:                cfg.Name,
		OutputDataDir:       cfg.DataDir,
		PeerURLs:            cfg.InitialAdvertisePeerURLs.StringSlice(),
		InitialCluster:      cfg.InitialCluster,
		InitialClusterToken: cfg.InitialClusterToken,
		SkipHashCheck:       cfg.SkipHashCheck,
	})
}

// InitialClusterFromMembers derives an "initial cluster" string from a member list
func InitialClusterFromMembers(members []*etcdserverpb.Member) string {
	namePeerURLs := []string{}
	for _, m := range members {
		for _, u := range m.PeerURLs {
			n := m.Name
			namePeerURLs = append(namePeerURLs, fmt.Sprintf("%s=%s", n, u))
		}
	}
	return strings.Join(namePeerURLs, ",")
}
