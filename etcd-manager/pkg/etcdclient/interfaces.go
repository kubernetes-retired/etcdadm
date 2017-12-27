package etcdclient

import (
	"context"
	"fmt"
	"strings"
)

type EtcdClient interface {
	Put(ctx context.Context, key string, value []byte) error
	Create(ctx context.Context, key string, value []byte) error
	Get(ctx context.Context, key string, quorum bool) ([]byte, error)
	CopyTo(ctx context.Context, dest EtcdClient) error
	ListMembers(ctx context.Context) ([]*EtcdProcessMember, error)
	AddMember(ctx context.Context, peerURLs []string) error
	RemoveMember(ctx context.Context, member *EtcdProcessMember) error
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
