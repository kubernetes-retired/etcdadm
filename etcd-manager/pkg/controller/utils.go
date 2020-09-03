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
