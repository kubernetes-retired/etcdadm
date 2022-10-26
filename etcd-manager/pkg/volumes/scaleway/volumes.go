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

package scaleway

import (
	"fmt"
	"os"

	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
	"k8s.io/klog/v2"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/volumes"
)

const (
	localDevicePrefix = "/dev/disk/by-id/scsi-0SCW_b_ssd_volume-"
)

// Volumes defines the Scaleway Cloud volume implementation.
type Volumes struct {
	clusterName string
	matchTags   []string
	nameTag     string

	scwClient   *scw.Client
	server      *instance.Server
	zone        scw.Zone
	instanceAPI *instance.API
}

var _ volumes.Volumes = &Volumes{}

// NewVolumes returns a new Scaleway Cloud volume provider.
func NewVolumes(clusterName string, volumeTags []string, nameTag string) (*Volumes, error) {
	scwClient, err := scw.NewClient(
		scw.WithEnv(),
	)
	if err != nil {
		return nil, fmt.Errorf("error creating Scaleway client: %w", err)
	}

	metadataAPI := instance.NewMetadataAPI()
	metadata, err := metadataAPI.GetMetadata()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve server metadata: %w", err)
	}

	serverID := metadata.ID
	klog.V(2).Infof("Found ID of the running server: %s", serverID)

	zoneID := metadata.Location.ZoneID
	zone, err := scw.ParseZone(zoneID)
	if err != nil {
		return nil, fmt.Errorf("unable to parse Scaleway zone: %w", err)
	}
	klog.V(2).Infof("Found zone of the running server: %v", zone)

	instanceAPI := instance.NewAPI(scwClient)
	server, err := instanceAPI.GetServer(&instance.GetServerRequest{
		ServerID: serverID,
		Zone:     zone,
	})
	if err != nil || server == nil {
		return nil, fmt.Errorf("failed to get the running server: %w", err)
	}
	klog.V(2).Infof("Found the running server: %q", server.Server.Name)

	a := &Volumes{
		clusterName: clusterName,
		matchTags:   volumeTags,
		nameTag:     nameTag,
		scwClient:   scwClient,
		server:      server.Server,
		zone:        zone,
		instanceAPI: instance.NewAPI(scwClient),
	}

	return a, nil
}

// FindVolumes returns all volumes that can be attached to the running server.
func (a *Volumes) FindVolumes() ([]*volumes.Volume, error) {
	klog.V(2).Infof("Finding attachable etcd volumes")

	etcdVolume, err := getAvailableVolume(a.instanceAPI, a.zone, a.matchTags)
	if err != nil {
		return nil, fmt.Errorf("failed to get matching volume: %w", err)
	}
	klog.V(2).Infof("Found attachable volume %s(%s) of type %s with status %q", etcdVolume.Name, etcdVolume.ID, etcdVolume.VolumeType, etcdVolume.State)

	localEtcdVolume := &volumes.Volume{
		ProviderID: etcdVolume.ID,
		Info: volumes.VolumeInfo{
			Description: a.clusterName + "-" + etcdVolume.ID,
		},
		MountName: "scw-" + etcdVolume.ID,
		EtcdName:  "vol-" + etcdVolume.ID,
	}

	if etcdVolume.Server != nil {
		localEtcdVolume.AttachedTo = etcdVolume.Server.ID
		localEtcdVolume.LocalDevice = fmt.Sprintf("%s%s", localDevicePrefix, etcdVolume.ID)
	}

	return []*volumes.Volume{localEtcdVolume}, nil
}

// FindMountedVolume returns the device where the volume is mounted to the running server.
func (a *Volumes) FindMountedVolume(volume *volumes.Volume) (string, error) {
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
func (a *Volumes) AttachVolume(volume *volumes.Volume) error {
	for {
		volumeResp, err := a.instanceAPI.GetVolume(&instance.GetVolumeRequest{
			VolumeID: volume.ProviderID,
			Zone:     a.zone,
		})
		if err != nil || volumeResp.Volume == nil {
			return fmt.Errorf("failed to get info for volume id %q: %w", volume.ProviderID, err)
		}

		// We check if the volume is already attached
		scwVolume := volumeResp.Volume
		if scwVolume.Server != nil {
			if scwVolume.Server.ID != a.server.ID {
				return fmt.Errorf("found volume %s(%s) attached to a different server: %s", scwVolume.Name, scwVolume.ID, scwVolume.Server.ID)
			}
			klog.V(2).Infof("Volume %s(%s) of type %q is already attached to the running server", scwVolume.Name, scwVolume.ID, scwVolume.VolumeType)
			volume.LocalDevice = fmt.Sprintf("%s%s", localDevicePrefix, scwVolume.ID)
			return nil
		}

		// We attach the volume to the server
		klog.V(2).Infof("Attaching volume %s(%s) of type %q to the running server", scwVolume.Name, scwVolume.ID, scwVolume.VolumeType)
		_, err = a.instanceAPI.AttachVolume(&instance.AttachVolumeRequest{
			Zone:     a.zone,
			ServerID: a.server.ID,
			VolumeID: scwVolume.ID,
		})
		if err != nil {
			return fmt.Errorf("failed to attach volume %s(%s): %w", scwVolume.Name, scwVolume.ID, err)
		}

		// We wait for the volume and the server to be in a stable state
		_, err = a.instanceAPI.WaitForVolume(&instance.WaitForVolumeRequest{
			VolumeID: scwVolume.ID,
			Zone:     a.zone,
		})
		if err != nil {
			return fmt.Errorf("error waiting for volume %s(%s): %w", scwVolume.Name, scwVolume.ID, err)
		}
		_, err = a.instanceAPI.WaitForServer(&instance.WaitForServerRequest{
			ServerID: a.server.ID,
			Zone:     a.zone,
		})
		if err != nil {
			return fmt.Errorf("error waiting for server %s(%s): %w", a.server.Name, a.server.ID, err)
		}

		// We fetch the volume again to check if the server is attached as expected
		volumeResp, err = a.instanceAPI.GetVolume(&instance.GetVolumeRequest{
			VolumeID: volume.ProviderID,
			Zone:     a.zone,
		})
		if err != nil || volumeResp.Volume == nil {
			return fmt.Errorf("failed to get info for volume id %q: %w", volume.ProviderID, err)
		}

		// We check that the attached volume is the one we expected, and we add the local device prefix
		scwVolume = volumeResp.Volume
		if scwVolume.Server != nil {
			if scwVolume.Server.ID != a.server.ID {
				return fmt.Errorf("found volume %s(%s) attached to a different server: %s", scwVolume.Name, scwVolume.ID, scwVolume.Server.ID)
			}
			klog.V(2).Infof("Volume %s(%s) of type %q is indeed attached to the running server", scwVolume.Name, scwVolume.ID, scwVolume.VolumeType)
			volume.LocalDevice = fmt.Sprintf("%s%s", localDevicePrefix, scwVolume.ID)
			return nil
		} else {
			return fmt.Errorf("XXX volume was not attached after operation XXX")
		}
	}
}

// MyIP returns the first private IP of the running server if successful.
func (a *Volumes) MyIP() (string, error) {
	if a.server.PrivateIP == nil || *a.server.PrivateIP == "" {
		return "", fmt.Errorf("failed to find private IP of server %s", a.server.ID)
	}
	klog.V(2).Infof("Found first private IP of the running server: %s", *a.server.PrivateIP)
	return *a.server.PrivateIP, nil
}

// getAvailableVolume returns the first available volume matching matchLabels if successful.
func getAvailableVolume(instanceAPI *instance.API, zone scw.Zone, matchLabels []string) (*instance.Volume, error) {
	matchingVolumes, err := instanceAPI.ListVolumes(&instance.ListVolumesRequest{
		Zone: zone,
		Tags: matchLabels,
	}, scw.WithAllPages())
	if err != nil {
		return nil, fmt.Errorf("failed to get volumes matching name %q: %w", matchLabels, err)
	}

	for _, volume := range matchingVolumes.Volumes {
		if volume.Server == nil && volume.State == instance.VolumeStateAvailable {
			klog.V(6).Infof("found 1 matching and available volume")
			return volume, nil
		}
	}

	return nil, fmt.Errorf("could not find any available volume matching %q: %w", matchLabels, err)
}

// getMatchingVolumes returns all the volumes matching matchLabels if successful.
func getMatchingVolumes(instanceAPI *instance.API, zone scw.Zone, matchLabels []string) ([]*instance.Volume, error) {
	matchingVolumes, err := instanceAPI.ListVolumes(&instance.ListVolumesRequest{
		Zone: zone,
		Tags: matchLabels,
	}, scw.WithAllPages())
	if err != nil {
		return nil, fmt.Errorf("failed to get volumes matching labels %q: %w", matchLabels, err)
	}
	klog.V(6).Infof("Got %d matching volumes", matchingVolumes.TotalCount)
	return matchingVolumes.Volumes, nil
}
