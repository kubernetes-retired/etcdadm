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

package legacy

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v2"
	"k8s.io/klog"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/pkg/commands"
	"kope.io/etcd-manager/pkg/etcdversions"
)

type legacyManifest struct {
	Spec legacyManifestSpec `json:"spec"`
}

type legacyManifestSpec struct {
	Containers []legacyManifestContainer `json:"containers"`
}

type legacyManifestContainer struct {
	Name  string              `json:"name"`
	Env   []legacyManifestEnv `json:"env"`
	Image string              `json:"image"`
}

type legacyManifestEnv struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (c *legacyManifestContainer) FindEnv(k string) string {
	for _, e := range c.Env {
		if e.Name == k {
			return e.Value
		}
	}
	return ""
}

var etcdKeys = []string{"etcd", "etcd-events"}

// ScanForExisting checks for existing data, and marks the cluster as created if found
// This stops the cluster being recreated when we're importing legacy data
func ScanForExisting(baseDir string, controlStore commands.Store) (bool, error) {
	for _, k := range etcdKeys {
		p := filepath.Join(baseDir, "k8s.io", "manifests", k+".manifest")
		b, err := ioutil.ReadFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return false, fmt.Errorf("error reading file %s: %v", p, err)
		}

		// Ignore empty files
		{
			if strings.TrimSpace(string(b)) == "" {
				klog.Infof("ignoring empty file %s", p)
				continue
			}
		}

		if err := controlStore.MarkClusterCreated(); err != nil {
			return false, fmt.Errorf("error marking cluster as created: %v", err)
		}

		return true, nil
	}
	return false, nil
}

// ImportExistingEtcd builds the state and copies the data from a legacy configuration
func ImportExistingEtcd(baseDir string, etcdNodeConfiguration *protoetcd.EtcdNode) (*protoetcd.EtcdState, error) {
	for _, k := range etcdKeys {
		p := filepath.Join(baseDir, "k8s.io", "manifests", k+".manifest")
		b, err := ioutil.ReadFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("error reading file %s: %v", p, err)
		}

		// Ignore empty files
		{
			if strings.TrimSpace(string(b)) == "" {
				klog.Infof("ignoring empty file %s", p)
				continue
			}
		}

		klog.Infof("parsing legacy manifest %s", p)

		m := &legacyManifest{}
		if err := yaml.Unmarshal(b, m); err != nil {
			return nil, fmt.Errorf("error parsing manifest %s: %v", p, err)
		}

		var etcdContainer *legacyManifestContainer
		for i := range m.Spec.Containers {
			if m.Spec.Containers[i].Name == "etcd-container" {
				etcdContainer = &m.Spec.Containers[i]
			}
		}

		if etcdContainer == nil {
			return nil, fmt.Errorf("etcd-container not found in manifest %s", p)
		}

		state := &protoetcd.EtcdState{
			NewCluster: false,
			Cluster:    &protoetcd.EtcdCluster{},
		}

		// Extract version from image
		{
			tokens := strings.Split(etcdContainer.Image, ":")
			if len(tokens) == 2 {
				state.EtcdVersion = tokens[1]
			} else if len(tokens) == 3 {
				// This is valid when pulling from proxies/mirrors with ports
				// localhost:8000/etcd:3.3.3
				state.EtcdVersion = tokens[2]
			} else {
				return nil, fmt.Errorf("unexpected etcd image %q", etcdContainer.Image)
			}
		}

		etcdName := strings.TrimSpace(etcdContainer.FindEnv("ETCD_NAME"))
		if etcdName == "" {
			return nil, fmt.Errorf("expected ETCD_NAME, was %q", etcdName)
		}

		etcdInitialCluster := strings.TrimSpace(etcdContainer.FindEnv("ETCD_INITIAL_CLUSTER"))
		for _, v := range strings.Split(etcdInitialCluster, ",") {
			tokens := strings.Split(v, "=")
			if len(tokens) != 2 {
				return nil, fmt.Errorf("cannot parse ETCD_INITIAL_CLUSTER %q", etcdInitialCluster)
			}

			node := &protoetcd.EtcdNode{
				Name:     tokens[0],
				PeerUrls: strings.Split(tokens[1], ","),
			}

			for _, peerURL := range node.PeerUrls {
				var peerSuffix, clientSuffix, quarantineSuffix string

				switch k {
				case "etcd":
					peerSuffix = ":2380"
					clientSuffix = ":4001"
					quarantineSuffix = ":3994"

				case "etcd-events":
					peerSuffix = ":2381"
					clientSuffix = ":4002"
					quarantineSuffix = ":3995"

				default:
					panic("unhandled legacy etcd")
				}

				if !strings.HasSuffix(peerURL, peerSuffix) {
					return nil, fmt.Errorf("unexpected peer url for %s: %q", k, peerURL)
				}

				if strings.HasPrefix(peerURL, "http://") {
					clientURL := "http://0.0.0.0" + clientSuffix
					quarantineURL := "http://0.0.0.0" + quarantineSuffix

					node.ClientUrls = append(node.ClientUrls, clientURL)
					node.QuarantinedClientUrls = append(node.QuarantinedClientUrls, quarantineURL)
				} else if strings.HasPrefix(peerURL, "https://") {
					clientURL := "https://0.0.0.0" + clientSuffix
					quarantineURL := "https://0.0.0.0" + quarantineSuffix

					node.ClientUrls = append(node.ClientUrls, clientURL)
					node.QuarantinedClientUrls = append(node.QuarantinedClientUrls, quarantineURL)
				} else {
					return nil, fmt.Errorf("scheme not yet implemented: %q", peerURL)
				}
			}

			state.Cluster.Nodes = append(state.Cluster.Nodes, node)
		}
		state.Cluster.DesiredClusterSize = int32(len(state.Cluster.Nodes))

		//state.Cluster.MyId = etcdName

		clusterToken := strings.TrimSpace(etcdContainer.FindEnv("ETCD_INITIAL_CLUSTER_TOKEN"))
		if clusterToken == "" {
			return nil, fmt.Errorf("expected ETCD_INITIAL_CLUSTER_TOKEN, was %q", etcdName)
		}
		state.Cluster.ClusterToken = clusterToken

		dataDir := filepath.Join(baseDir, "data", clusterToken)

		var legacyDir string
		switch k {
		case "etcd":
			legacyDir = filepath.Join(baseDir, "var", "etcd", "data")
		case "etcd-events":
			legacyDir = filepath.Join(baseDir, "var", "etcd", "data-events")
		default:
			panic("unhandled legacy etcd")
		}

		klog.Infof("copying etcd data from %s -> %s", legacyDir, dataDir)
		if err := copyDir(legacyDir, dataDir); err != nil {
			return nil, err
		}

		trashcanDir := filepath.Join(baseDir, "data-trashcan")
		newDataDir := filepath.Join(trashcanDir, clusterToken)
		klog.Infof("archiving etcd data directory %s -> %s", legacyDir, newDataDir)
		if err := os.MkdirAll(trashcanDir, 0755); err != nil {
			return nil, fmt.Errorf("error creating trashcan directory %s: %v", trashcanDir, err)
		}
		if err := os.Rename(legacyDir, newDataDir); err != nil {
			return nil, fmt.Errorf("error renaming directory %s -> %s: %v", legacyDir, newDataDir, err)
		}

		adoptAs := etcdversions.EtcdVersionForAdoption(state.EtcdVersion)
		if adoptAs != "" && adoptAs != state.EtcdVersion {
			klog.Warningf("adopting from etcd %q, will adopt with %q", state.EtcdVersion, adoptAs)
			state.EtcdVersion = adoptAs
		}

		klog.Infof("imported etcd state: %v", state.String())
		return state, nil
	}

	return nil, nil
}

func copyDir(src, dest string) error {
	if err := os.MkdirAll(dest, 0755); err != nil {
		return fmt.Errorf("error creating directories %s: %v", dest, err)
	}
	paths, err := ioutil.ReadDir(src)
	if err != nil {
		return fmt.Errorf("error reading source directory %s: %v", src, err)
	}

	for _, p := range paths {
		srcFile := filepath.Join(src, p.Name())
		destFile := filepath.Join(dest, p.Name())

		if p.IsDir() {
			if err := copyDir(srcFile, destFile); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcFile, destFile); err != nil {
				return fmt.Errorf("error copying %s -> %s: %v", srcFile, destFile, err)
			}
		}
	}

	return nil
}

func copyFile(srcfile, destfile string) (err error) {
	in, err := os.Open(srcfile)
	if err != nil {
		return
	}
	defer func() {
		cerr := in.Close()
		if err == nil {
			err = cerr
		}
	}()
	out, err := os.Create(destfile)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	return
}
