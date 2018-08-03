package snapshot

import (
	"github.com/coreos/etcd/snapshot"
	"github.com/platform9/etcdadm/apis"
	"go.uber.org/zap"
)

// Restore initializes the etcd member from a snapshot
func Restore(cfg *apis.EtcdAdmConfig) error {
	var lg *zap.Logger
	sp := snapshot.NewV3(lg)

	return sp.Restore(snapshot.RestoreConfig{
		SnapshotPath:        cfg.Snapshot,
		Name:                cfg.Name,
		OutputDataDir:       cfg.DataDir,
		PeerURLs:            cfg.AdvertisePeerURLs.StringSlice(),
		InitialCluster:      cfg.InitialCluster,
		InitialClusterToken: cfg.InitialClusterToken,
		SkipHashCheck:       cfg.SkipHashCheck,
	})
}
