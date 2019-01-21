package fs

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/golang/glog"
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

	glog.Infof("publishing discovery record: %v", d.me)

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
	glog.V(2).Infof("polling discovery directory: %s", d.base)
	nodes := make(map[string]discovery.Node)

	files, err := d.base.ReadDir()
	if err != nil {
		return nil, fmt.Errorf("error reading directory %s: %v", d.base, err)
	}

	for _, p := range files {
		id := p.Base()

		data, err := p.ReadFile()
		if err != nil {
			glog.Warningf("error reading node discovery file %s: %v", p, err)
			continue
		}

		node := discovery.Node{}
		if err := json.Unmarshal(data, &node); err != nil {
			glog.Warningf("error parsing node discovery file %s: %v", p, err)
			continue
		}

		nodes[id] = node
	}

	return nodes, nil
}
