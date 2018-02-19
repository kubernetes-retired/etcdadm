package etcdclient

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// EtcdClient is an abstract client for V2 and V3
type EtcdClient interface {
	io.Closer

	Put(ctx context.Context, key string, value []byte) error
	Create(ctx context.Context, key string, value []byte) error

	// Get returns the value of the specified key, or (nil, nil) if not found
	Get(ctx context.Context, key string, quorum bool) ([]byte, error)

	CopyTo(ctx context.Context, dest EtcdClient) (int, error)
	ListMembers(ctx context.Context) ([]*EtcdProcessMember, error)
	AddMember(ctx context.Context, peerURLs []string) error
	RemoveMember(ctx context.Context, member *EtcdProcessMember) error

	// ServerVersion returns the version of etcd running
	ServerVersion(ctx context.Context) (string, error)

	// LocalNodeInfo returns information about the etcd member node we are connected to
	LocalNodeInfo(ctx context.Context) (*LocalNodeInfo, error)
}

// LocalNodeInfo has information about the etcd member node we are connected to
type LocalNodeInfo struct {
	IsLeader bool
}

func NewClient(etcdVersion string, clientURLs []string) (EtcdClient, error) {
	if IsV2(etcdVersion) {
		return NewV2Client(clientURLs)
	}
	if IsV3(etcdVersion) {
		return NewV3Client(clientURLs)
	}
	return nil, fmt.Errorf("unhandled etcd version %q", etcdVersion)
}

// IsV2 returns true if the specified etcdVersion is a 2.x version
func IsV2(etcdVersion string) bool {
	return strings.HasPrefix(etcdVersion, "2.")
}

// IsV3 returns true if the specified etcdVersion is a 3.x version
func IsV3(etcdVersion string) bool {
	return strings.HasPrefix(etcdVersion, "3.")
}
