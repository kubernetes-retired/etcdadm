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

package commands

import (
	"k8s.io/kops/util/pkg/vfs"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
)

const CommandFilename = "_command.json"

type Store interface {
	// IsNewCluster indicates if it is safe to create a new cluster
	IsNewCluster() (bool, error)

	// MarkClusterCreated marks the cluster as having been created, so IsNewCluster will return false
	MarkClusterCreated() error

	// GetExpectedClusterSpec gets the expected cluster spec
	GetExpectedClusterSpec() (*protoetcd.ClusterSpec, error)
	// SetExpectedClusterSpec updates the expected cluster spec
	SetExpectedClusterSpec(spec *protoetcd.ClusterSpec) error

	// AddCommand adds a command to the back of the queue
	AddCommand(cmd *protoetcd.Command) error

	// ListCommands returns all the external commands that have not been removed
	ListCommands() ([]Command, error)

	// RemoveCommand marks a command as complete
	RemoveCommand(command Command) error
}

type Command interface {
	Data() protoetcd.Command
}

func NewStore(storage string) (Store, error) {
	p, err := vfs.Context.BuildVfsPath(storage)
	if err != nil {
		return nil, err
	}
	return NewVFSStore(p)
}
