package etcd

import (
	"fmt"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/coreos/etcd/snapshot"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/platform9/etcdadm/apis"
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
func MemberForPeerURLs(members []*etcdserverpb.Member, peerURLs []string) (bool, *etcdserverpb.Member) {
	for _, m := range members {
		if cmp.Equal(m.PeerURLs, peerURLs, cmpopts.EquateEmpty()) {
			return true, m
		}
	}
	return false, nil
}

// MemberForID searches the list for a member with a matching ID.
func MemberForID(members []*etcdserverpb.Member, id uint64) (bool, *etcdserverpb.Member) {
	for _, m := range members {
		if m.ID == id {
			return true, m
		}
	}
	return false, nil
}

// Started checks whether the member has started.
func Started(member etcdserverpb.Member) bool {
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
