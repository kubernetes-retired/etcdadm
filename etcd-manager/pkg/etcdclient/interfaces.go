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
	"io"

	"k8s.io/klog/v2"
)

// EtcdClient is an abstract client for V2 and V3
type EtcdClient interface {
	io.Closer

	Put(ctx context.Context, key string, value []byte) error
	Create(ctx context.Context, key string, value []byte) error

	// Get returns the value of the specified key, or (nil, nil) if not found
	Get(ctx context.Context, key string, quorum bool) ([]byte, error)

	// CopyTo traverses every key and writes it to dest
	CopyTo(ctx context.Context, dest NodeSink) (int, error)

	// LeaderID returns the ID of the current leader, or "" if there is no leader
	// NOTE: This is currently only used in end-to-end tests
	LeaderID(ctx context.Context) (string, error)

	ListMembers(ctx context.Context) ([]*EtcdProcessMember, error)
	AddMember(ctx context.Context, peerURLs []string) error
	RemoveMember(ctx context.Context, member *EtcdProcessMember) error

	SetPeerURLs(ctx context.Context, member *EtcdProcessMember, peerURLs []string) error

	// ServerVersion returns the version of etcd running
	ServerVersion(ctx context.Context) (string, error)

	// LocalNodeInfo returns information about the etcd member node we are connected to
	LocalNodeInfo(ctx context.Context) (*LocalNodeInfo, error)

	// SnapshotSave makes a snapshot (backup) of the data in path.  Only supported in V3.
	SnapshotSave(ctx context.Context, path string) error

	// SupportsSnapshot checks if the Snapshot method is supported (i.e. if we are V3)
	SupportsSnapshot() bool
}

// NodeSink is implemented by a target for CopyTo
type NodeSink interface {
	io.Closer

	Put(ctx context.Context, key string, value []byte) error
}

// LocalNodeInfo has information about the etcd member node we are connected to
type LocalNodeInfo struct {
	IsLeader bool
}

func NewClient(clientURLs []string, tlsConfig *tls.Config) (EtcdClient, error) {
	if len(clientURLs) == 0 {
		return nil, fmt.Errorf("no client URLs were provided")
	}

	return NewV3Client(clientURLs, tlsConfig)
}

// LoggedClose closes the etcdclient, warning on error
func LoggedClose(etcdClient EtcdClient) {
	if err := etcdClient.Close(); err != nil {
		klog.Warningf("error closing etcd client: %v", err)
	}
}
