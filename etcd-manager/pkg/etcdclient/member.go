package etcdclient

import (
	"encoding/json"
	"fmt"
)

type EtcdProcessMember struct {
	//Id         string   `json:"id,omitempty"`
	Name       string   `json:"name,omitempty"`
	PeerURLs   []string `json:"peerURLs,omitempty"`
	ClientURLs []string `json:"clientURLs,omitempty"`

	etcdVersion string

	ID   string
	idv2 string
	idv3 uint64
}

func (m *EtcdProcessMember) NewClient() (EtcdClient, error) {
	return NewClient(m.etcdVersion, m.ClientURLs)
}

func (m *EtcdProcessMember) String() string {
	s, err := json.Marshal(m)
	if err != nil {
		return fmt.Sprintf("<error marshallling: %v>", err)
	}
	return string(s)
}
