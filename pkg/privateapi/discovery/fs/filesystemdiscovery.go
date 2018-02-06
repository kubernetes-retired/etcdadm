package fs

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/golang/glog"

	"kope.io/etcd-manager/pkg/privateapi/discovery"
)

// FilesystemDiscovery implements discovery.Interface using a shared directory.
// This is primarily for testing.
type FilesystemDiscovery struct {
	baseDir string
	me      discovery.Node
}

var _ discovery.Interface = &FilesystemDiscovery{}

func NewFilesystemDiscovery(baseDir string, me discovery.Node) (*FilesystemDiscovery, error) {
	d := &FilesystemDiscovery{
		baseDir: baseDir,
		me:      me,
	}

	err := d.publish()
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *FilesystemDiscovery) publish() error {
	if d.me.ID == "" {
		return fmt.Errorf("DiscoveryNode does not have ID set")
	}

	glog.Infof("publishing discovery record: %v", d.me)

	meJson, err := json.Marshal(d.me)
	if err != nil {
		return fmt.Errorf("error marshalling to JSON: %v", err)
	}

	if err := os.MkdirAll(d.baseDir, 0755); err != nil {
		glog.Warningf("unable to mkdir %s: %v", d.baseDir, err)
	}

	p := filepath.Join(d.baseDir, string(d.me.ID))
	if err := ioutil.WriteFile(p, meJson, 0755); err != nil {
		return fmt.Errorf("error writing file %s: %v", p, err)
	}

	return nil
}

func (d *FilesystemDiscovery) Poll() (map[string]discovery.Node, error) {
	glog.V(2).Infof("polling discovery directory: %s", d.baseDir)
	nodes := make(map[string]discovery.Node)

	files, err := ioutil.ReadDir(d.baseDir)
	if err != nil {
		return nil, fmt.Errorf("error reading directory %s: %v", d.baseDir, err)
	}

	for _, f := range files {
		id := f.Name()

		p := filepath.Join(d.baseDir, id)
		data, err := ioutil.ReadFile(p)
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
