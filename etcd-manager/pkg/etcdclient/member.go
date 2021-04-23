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
	"crypto/tls"
	"encoding/json"
	"fmt"
)

type EtcdProcessMember struct {
	//Id         string   `json:"id,omitempty"`
	Name     string   `json:"name,omitempty"`
	PeerURLs []string `json:"peerURLs,omitempty"`
	// ClientURLs is the set of URLs as reported by the cluster.
	// Note that it might be incorrect, because the ClientURLs are stored in Raft, but can be reconfigured from the command line
	ClientURLs []string `json:"endpoints,omitempty"`

	etcdVersion string

	ID   string
	idv2 string
	idv3 uint64
}

func (m *EtcdProcessMember) NewClient(clientURLs []string, tlsConfig *tls.Config) (*EtcdClient, error) {
	return NewClient(clientURLs, tlsConfig)
}

func (m *EtcdProcessMember) String() string {
	s, err := json.Marshal(m)
	if err != nil {
		return fmt.Sprintf("<error marshallling: %v>", err)
	}
	return string(s)
}
