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

type Volumes interface {
	AttachVolume(volume *Volume) error
	FindVolumes() ([]*Volume, error)

	// FindMountedVolume returns the device (e.g. /dev/sda) where the volume is mounted
	// If not found, it returns "", nil
	// On error, it returns "", err
	FindMountedVolume(volume *Volume) (device string, err error)

	// MyIP returns the current node's IP address
	MyIP() (string, error)
}

type Volume struct {
	// ProviderID is the cloud-provider identifier for the volume
	ProviderID string

	// EtcdName is the name for etcd
	EtcdName string

	// LocalDevice is set if the volume is attached to the local machine
	LocalDevice string

	// AttachedTo is set to the ID of the machine the volume is attached to, or "" if not attached
	AttachedTo string

	// Mountpoint is the path on which the volume is mounted, if mounted
	// It will likely be "/mnt/master-" + ID
	Mountpoint string

	// Status is a volume provider specific Status string; it makes it easier for the volume provider
	Status string

	Info VolumeInfo
}

type VolumeInfo struct {
	Description string
}
