/*
Copyright 2019 The Kubernetes Authors.

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

package external

import (
	"fmt"
	"io/ioutil"
	"net"
	"strings"

	"github.com/golang/glog"

	"kope.io/etcd-manager/pkg/privateapi/discovery"
)

// ExternalDiscovery also allows us to discover our peer nodes
var _ discovery.Interface = &ExternalDiscovery{}

type ExternalDiscovery struct {
	seeddir string
}

func NewExternalDiscovery(seeddir string) *ExternalDiscovery {
	return &ExternalDiscovery{seeddir: seeddir}
}

func (a *ExternalDiscovery) Poll() (map[string]discovery.Node, error) {
	nodes := make(map[string]discovery.Node)

	files, err := ioutil.ReadDir(a.seeddir)
	if err != nil {
		return nil, fmt.Errorf("error reading seed directory %s: %v", a.seeddir, err)
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}

		ip := net.ParseIP(f.Name())
		if ip == nil {
			glog.Infof("ignoring unknown seed file %q (expected IP)", f.Name())
			continue
		}

		ipString := ip.String()
		// We use the IP as the persistent identifier, because we don't expect these to move around as much
		id := "ip-" + strings.ReplaceAll(ipString, ".", "-")
		node := discovery.Node{
			ID: id,
		}
		node.Endpoints = append(node.Endpoints, discovery.NodeEndpoint{IP: ipString})
		nodes[node.ID] = node
	}

	return nodes, nil
}
