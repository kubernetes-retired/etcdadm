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

package fs

import (
	"bytes"
	"encoding/json"
	"fmt"

	"k8s.io/klog"
	"k8s.io/kops/util/pkg/vfs"

	"kope.io/etcd-manager/pkg/privateapi/discovery"
)

// VFSDiscovery implements discovery.Interface using a vfs.Path
// This is primarily for testing.
type VFSDiscovery struct {
	base vfs.Path
	me   discovery.Node
}

var _ discovery.Interface = &VFSDiscovery{}

func NewVFSDiscovery(base vfs.Path, me discovery.Node) (*VFSDiscovery, error) {
	d := &VFSDiscovery{
		base: base,
		me:   me,
	}

	err := d.publish()
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *VFSDiscovery) publish() error {
	if d.me.ID == "" {
		return fmt.Errorf("DiscoveryNode does not have ID set")
	}

	klog.Infof("publishing discovery record: %v", d.me)

	meJson, err := json.Marshal(d.me)
	if err != nil {
		return fmt.Errorf("error marshalling to JSON: %v", err)
	}

	p := d.base.Join(string(d.me.ID))
	if err := p.WriteFile(bytes.NewReader(meJson), nil); err != nil {
		return fmt.Errorf("error writing file %s: %v", p, err)
	}

	return nil
}

func (d *VFSDiscovery) Poll() (map[string]discovery.Node, error) {
	klog.V(2).Infof("polling discovery directory: %s", d.base)
	nodes := make(map[string]discovery.Node)

	files, err := d.base.ReadDir()
	if err != nil {
		return nil, fmt.Errorf("error reading directory %s: %v", d.base, err)
	}

	for _, p := range files {
		id := p.Base()

		data, err := p.ReadFile()
		if err != nil {
			klog.Warningf("error reading node discovery file %s: %v", p, err)
			continue
		}

		node := discovery.Node{}
		if err := json.Unmarshal(data, &node); err != nil {
			klog.Warningf("error parsing node discovery file %s: %v", p, err)
			continue
		}

		nodes[id] = node
	}

	return nodes, nil
}
