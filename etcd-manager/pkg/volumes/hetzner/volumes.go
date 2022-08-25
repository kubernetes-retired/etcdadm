/*
Copyright 2022 The Kubernetes Authors.

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

package hetzner

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/hetznercloud/hcloud-go/hcloud/metadata"
	"k8s.io/klog/v2"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/volumes"
)

const (
	hcloudTokenENVVar = "HCLOUD_TOKEN"
	localDevicePrefix = "/dev/disk/by-id/scsi-0HC_Volume_"
)

// HetznerVolumes defines the Hetzner Cloud volume implementation.
type HetznerVolumes struct {
	clusterName string

	matchPeerTags map[string]string
	matchNameTags map[string]string

	hcloudClient *hcloud.Client
	server       *hcloud.Server
}

var _ volumes.Volumes = &HetznerVolumes{}

// NewHetznerVolumes returns a new Hetzner Cloud volume provider.
func NewHetznerVolumes(clusterName string, volumeTags []string, nameTag string) (*HetznerVolumes, error) {
	serverID, err := metadata.NewClient().InstanceID()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve server id: %w", err)
	}
	klog.V(2).Infof("Found ID of the running server: %d", serverID)

	hcloudToken := os.Getenv(hcloudTokenENVVar)
	if hcloudToken == "" {
		return nil, fmt.Errorf("%s is required", hcloudTokenENVVar)
	}
	opts := []hcloud.ClientOption{
		hcloud.WithToken(hcloudToken),
	}
	hcloudClient := hcloud.NewClient(opts...)

	server, _, err := hcloudClient.Server.GetByID(context.TODO(), serverID)
	if err != nil || server == nil {
		return nil, fmt.Errorf("failed to get info for the running server: %w", err)
	}

	klog.V(2).Infof("Found name of the running server: %q", server.Name)
	if server.Datacenter != nil && server.Datacenter.Location != nil {
		klog.V(2).Infof("Found location of the running server: %q", server.Datacenter.Location.Name)
	} else {
		return nil, fmt.Errorf("failed to find location of the running server")
	}
	if len(server.PrivateNet) > 0 {
		klog.V(2).Infof("Found first private net IP of the running server: %q", server.PrivateNet[0].IP.String())
	} else {
		return nil, fmt.Errorf("failed to find private net of the running server")
	}

	a := &HetznerVolumes{
		clusterName:   clusterName,
		matchPeerTags: make(map[string]string),
		matchNameTags: make(map[string]string),
		hcloudClient:  hcloudClient,
		server:        server,
	}

	for _, volumeTag := range volumeTags {
		tokens := strings.SplitN(volumeTag, "=", 2)
		if len(tokens) == 1 {
			a.matchPeerTags[tokens[0]] = ""
			a.matchNameTags[tokens[0]] = ""
		} else {
			a.matchPeerTags[tokens[0]] = tokens[1]
			a.matchNameTags[tokens[0]] = tokens[1]
		}
	}

	{
		tokens := strings.SplitN(nameTag, "=", 2)
		if len(tokens) == 1 {
			a.matchNameTags[tokens[0]] = ""
		} else {
			a.matchNameTags[tokens[0]] = tokens[1]
		}
	}

	return a, nil
}

// FindVolumes returns all volumes that can be attached to the running server.
func (a *HetznerVolumes) FindVolumes() ([]*volumes.Volume, error) {
	klog.V(2).Infof("Finding attachable etcd volumes")

	if a.server.Datacenter == nil || a.server.Datacenter.Location == nil {
		return nil, fmt.Errorf("failed to find server location for the running server")
	}
	serverLocation := a.server.Datacenter.Location.Name

	matchingVolumes, err := getMatchingVolumes(a.hcloudClient, a.matchNameTags)
	if err != nil {
		return nil, fmt.Errorf("failed to get matching volumes: %w", err)
	}

	var localEtcdVolumes []*volumes.Volume
	for _, volume := range matchingVolumes {
		// Only volumes from the same location can be mounted
		if volume.Location == nil {
			klog.Warningf("failed to find volume %s(%d) location", volume.Name, volume.ID)
			continue
		}
		volumeLocation := volume.Location.Name
		if volumeLocation != serverLocation {
			continue
		}

		klog.V(2).Infof("Found attachable volume %s(%d) with status %q", volume.Name, volume.ID, volume.Status)

		volumeID := strconv.Itoa(volume.ID)
		localEtcdVolume := &volumes.Volume{
			ProviderID: volumeID,
			Info: volumes.VolumeInfo{
				Description: a.clusterName + "-" + volumeID,
			},
			MountName: "hcloud-" + volumeID,
			EtcdName:  "vol-" + volumeID,
		}

		if volume.Server != nil {
			localEtcdVolume.AttachedTo = strconv.Itoa(volume.Server.ID)
			if volume.Server.ID == a.server.ID {
				localEtcdVolume.LocalDevice = fmt.Sprintf("%s%d", localDevicePrefix, volume.ID)
			}
		}

		localEtcdVolumes = append(localEtcdVolumes, localEtcdVolume)
	}

	return localEtcdVolumes, nil
}

// FindMountedVolume returns the device where the volume is mounted to the running server.
func (a *HetznerVolumes) FindMountedVolume(volume *volumes.Volume) (string, error) {
	device := volume.LocalDevice

	klog.V(2).Infof("Finding mounted volume %q", device)
	_, err := os.Stat(volumes.PathFor(device))
	if err == nil {
		klog.V(2).Infof("Found mounted volume %q", device)
		return device, nil
	}

	if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to find local device %q: %w", device, err)
	}

	// When not found, the interface says to return ("", nil)
	return "", nil
}

// AttachVolume attaches the specified volume to the running server and returns the mountpoint if successful.
func (a *HetznerVolumes) AttachVolume(volume *volumes.Volume) error {
	volumeID, err := strconv.Atoi(volume.ProviderID)
	if err != nil {
		return fmt.Errorf("failed to convert volume id %q to int: %w", volume.ProviderID, err)
	}

	for {
		hetznerVolume, _, err := a.hcloudClient.Volume.GetByID(context.TODO(), volumeID)
		if err != nil || hetznerVolume == nil {
			return fmt.Errorf("failed to get info for volume id %q: %w", volume.ProviderID, err)
		}

		if hetznerVolume.Server != nil {
			if hetznerVolume.Server.ID != a.server.ID {
				return fmt.Errorf("found volume %s(%d) attached to a different server: %d", hetznerVolume.Name, hetznerVolume.ID, hetznerVolume.Server.ID)
			}

			klog.V(2).Infof("Attached volume %s(%d) to the running server", hetznerVolume.Name, hetznerVolume.ID)

			volume.LocalDevice = fmt.Sprintf("%s%d", localDevicePrefix, hetznerVolume.ID)
			return nil
		}

		klog.V(2).Infof("Attaching volume %s(%d) to the running server", hetznerVolume.Name, hetznerVolume.ID)

		action, _, err := a.hcloudClient.Volume.Attach(context.TODO(), hetznerVolume, a.server)
		if err != nil {
			return fmt.Errorf("failed to attach volume %s(%d): %w", hetznerVolume.Name, hetznerVolume.ID, err)
		}
		if action.Status != hcloud.ActionStatusRunning && action.Status != hcloud.ActionStatusSuccess {
			return fmt.Errorf("found invalid status for volume %s(%d): %s", hetznerVolume.Name, hetznerVolume.ID, action.Status)
		}

		time.Sleep(10 * time.Second)
	}
}

// MyIP returns the first private IP of the running server if successful.
func (a *HetznerVolumes) MyIP() (string, error) {
	if len(a.server.PrivateNet) == 0 {
		return "", fmt.Errorf("failed to find server private ip address")
	}

	klog.V(2).Infof("Found first private IP of the running server: %s", a.server.PrivateNet[0].IP.String())

	return a.server.PrivateNet[0].IP.String(), nil
}

// getMatchingVolumes returns all the volumes matching matchLabels if successful.
func getMatchingVolumes(client *hcloud.Client, matchLabels map[string]string) ([]*hcloud.Volume, error) {
	var labels []string
	for k, v := range matchLabels {
		if v == "" {
			labels = append(labels, k)
		} else {
			labels = append(labels, k+"="+v)
		}
	}
	labelSelector := strings.Join(labels, ",")
	listOptions := hcloud.ListOpts{
		PerPage:       50,
		LabelSelector: labelSelector,
	}
	volumeListOptions := hcloud.VolumeListOpts{ListOpts: listOptions}
	matchingVolumes, err := client.Volume.AllWithOpts(context.TODO(), volumeListOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to get volumes matching label selector %q: %w", labelSelector, err)
	}

	return matchingVolumes, nil
}
