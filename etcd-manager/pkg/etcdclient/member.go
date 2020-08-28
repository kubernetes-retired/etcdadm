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

func (m *EtcdProcessMember) NewClient(clientURLs []string, tlsConfig *tls.Config) (EtcdClient, error) {
	return NewClient(m.etcdVersion, clientURLs, tlsConfig)
}

func (m *EtcdProcessMember) String() string {
	s, err := json.Marshal(m)
	if err != nil {
		return fmt.Sprintf("<error marshallling: %v>", err)
	}
	return string(s)
}
