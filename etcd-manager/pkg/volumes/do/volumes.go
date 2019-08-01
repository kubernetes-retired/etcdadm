/*
Copyright 2016 The Kubernetes Authors.

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

package do

import (
	//"fmt"
	// "os"
	// "path/filepath"
	// "strings"
	// "sync"
	// "time"

	"github.com/digitalocean/godo"

	// "github.com/golang/glog"

	"kope.io/etcd-manager/pkg/volumes"
)

// DOVolumes defines the digital ocean's volume implementation
type DOVolumes struct {
	ClusterID string
	//Cloud     *digitalocean.Cloud
	Client *godo.Client
	region      string
	dropletName string
	dropletID   int
}

var _ volumes.Volumes = &DOVolumes{}

func NewDOVolumes(clusterName string, volumeTags []string, nameTag string) (*DOVolumes, error) {
	return nil, nil
}

func (a *DOVolumes) FindVolumes() ([]*volumes.Volume, error) {
	return nil, nil
}

// FindMountedVolume implements Volumes::FindMountedVolume
func (a *DOVolumes) FindMountedVolume(volume *volumes.Volume) (string, error) {
	return "", nil
}

// AttachVolume attaches the specified volume to this instance, returning the mountpoint & nil if successful
func (a *DOVolumes) AttachVolume(volume *volumes.Volume) error {
	return nil
}

func (a *DOVolumes) MyIP() (string, error) {
	return "", nil
}

func getAllVolumesByRegion(cloud *godo.Client, region string) ([]godo.Volume, error) {
	return nil, nil
}
