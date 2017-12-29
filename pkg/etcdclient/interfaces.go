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

	CopyTo(ctx context.Context, dest EtcdClient) error
	ListMembers(ctx context.Context) ([]*EtcdProcessMember, error)
	AddMember(ctx context.Context, peerURLs []string) error
	RemoveMember(ctx context.Context, member *EtcdProcessMember) error

	// ServerVersion returns the version of etcd running
	ServerVersion(ctx context.Context) (string, error)
}

func NewClient(etcdVersion string, clientURLs []string) (EtcdClient, error) {
	if strings.HasPrefix(etcdVersion, "2.") {
		return NewV2Client(clientURLs)
	}
	if strings.HasPrefix(etcdVersion, "3.") {
		return NewV3Client(clientURLs)
	}
	return nil, fmt.Errorf("unhandled etcd version %q", etcdVersion)
}
