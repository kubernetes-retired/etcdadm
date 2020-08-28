package controller

import (
	"strings"

	"kope.io/etcd-manager/pkg/etcdclient"
)

func isTLSEnabled(member *etcdclient.EtcdProcessMember) bool {
	tlsEnabled := false
	for _, u := range member.ClientURLs {
		if strings.HasPrefix(u, "https://") {
			tlsEnabled = true
		}
	}
	for _, u := range member.PeerURLs {
		if strings.HasPrefix(u, "https://") {
			tlsEnabled = true
		}
	}
	return tlsEnabled
}
