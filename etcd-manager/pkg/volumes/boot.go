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

package volumes

import (
	"time"

	"k8s.io/klog/v2"
)

var (
	// Containerized indicates the etcd is containerized
	Containerized = false
)

// Boot s ithe options for the protokube service
type Boot struct {
	volumeMounter *VolumeMountController
}

// Init is responsible for initializing the controllers
func (b *Boot) Init(volumesProvider Volumes) {
	b.volumeMounter = newVolumeMountController(volumesProvider)
}

func (b *Boot) WaitForVolumes() []*Volume {
	for {
		info, err := b.tryMountVolumes()
		if err != nil {
			klog.Warningf("error during attempt to bootstrap (will sleep and retry): %v", err)
			time.Sleep(1 * time.Second)
			continue
		} else if len(info) != 0 {
			return info
		} else {
			klog.Infof("waiting for volumes")
		}

		time.Sleep(1 * time.Minute)
	}
}

func (b *Boot) tryMountVolumes() ([]*Volume, error) {
	// attempt to mount the volumes
	volumes, err := b.volumeMounter.mountMasterVolumes()
	if err != nil {
		return nil, err
	}

	return volumes, nil
}

func PathFor(hostPath string) string {
	if hostPath[0] != '/' {
		klog.Fatalf("path was not absolute: %q", hostPath)
	}
	rootfs := "/"
	if Containerized {
		rootfs = "/rootfs/"
	}
	return rootfs + hostPath[1:]
}
