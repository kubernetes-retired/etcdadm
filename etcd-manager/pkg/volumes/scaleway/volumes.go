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
	localDevicePrefix = "/dev/disk/by-id/scsi-0HC_Volume_"
)

// ScwVolumes defines the Scaleway Cloud volume implementation.
type ScwVolumes struct {
	clusterName string
	matchTags   []string
	nameTag     string

	scwClient   *scw.Client
	server      *instance.Server
	zone        scw.Zone
	instanceAPI *instance.API
}

var _ volumes.Volumes = &ScwVolumes{}

// NewScwVolumes returns a new Scaleway Cloud volume provider.
func NewScwVolumes(clusterName string, volumeTags []string, nameTag string) (*ScwVolumes, error) {
	scwClient, err := scw.NewClient(
		scw.WithEnv(),
	)
	if err != nil {
		return nil, err
	}

	//metadataAPI := instance.NewMetadataAPI()
	//metadata, err := metadataAPI.GetMetadata()
	//if err != nil {
	//	return nil, fmt.Errorf("failed to retrieve server metadata: %s", err)
	//}
	//TODO(Mia-Cross): The previous segment only works if launched from instance
	// For developpement purposes, we can execute the following code instead
	master, err := instance.NewAPI(scwClient).ListServers(&instance.ListServersRequest{
		Zone: scw.Zone(os.Getenv("SCW_DEFAULT_ZONE")),
		Tags: []string{"instance-group=master-fr-par-1"},
	})
	metadata := master.Servers[0]

	serverID := metadata.ID
	klog.V(2).Infof("Found ID of the running server: %v", serverID)

	zoneID := metadata.Location.ZoneID
	zone, err := scw.ParseZone(zoneID)
	if err != nil {
		return nil, fmt.Errorf("unable to parse Scaleway zone: %s", err)
	}
	klog.V(2).Infof("Found zone of the running server: %v", zone)
	//privateIP := metadata.PrivateIP
	//klog.V(2).Infof("Found first private net IP of the running server: %q", privateIP)

	instanceAPI := instance.NewAPI(scwClient)
	server, err := instanceAPI.GetServer(&instance.GetServerRequest{
		ServerID: serverID,
		Zone:     zone,
	})
	if err != nil || server == nil {
		return nil, fmt.Errorf("failed to get the running server: %s", err)
	}
	klog.V(2).Infof("Found the running server: %q", server.Server.Name)

	a := &ScwVolumes{
		clusterName: clusterName,
		matchTags:   volumeTags,
		nameTag:     nameTag,
		scwClient:   scwClient,
		server:      server.Server,
		zone:        zone,
		instanceAPI: instance.NewAPI(scwClient),
	}

	//for _, volumeTag := range volumeTags {
	//	tokens := strings.SplitN(volumeTag, "=", 2)
	//	if len(tokens) == 1 {
	//		a.matchTags[tokens[0]] = ""
	//	} else {
	//		a.matchTags[tokens[0]] = tokens[1]
	//	}
	//}

	return a, nil
}

// FindVolumes returns all volumes that can be attached to the running server.
func (a *ScwVolumes) FindVolumes() ([]*volumes.Volume, error) {
	klog.V(2).Infof("Finding attachable etcd volumes")

	allEtcdVolumes, err := getMatchingVolumes(a.instanceAPI, a.zone, a.matchTags)
	if err != nil {
		return nil, fmt.Errorf("failed to get matching volumes: %s", err)
	}

	var localEtcdVolumes []*volumes.Volume
	for _, volume := range allEtcdVolumes {
		// Only volumes from the same location can be mounted
		if volume.Zone == "" {
			klog.Warningf("failed to find volume location for %s(%d)", volume.Name, volume.ID)
			continue
		}
		volumeLocation := volume.Zone
		if volumeLocation != a.zone {
			continue
		}
		klog.V(2).Infof("Found attachable volume %s(%d) of type %s with status %q", volume.Name, volume.ID, volume.VolumeType, volume.State)

		localEtcdVolume := &volumes.Volume{
			ProviderID: volume.ID,
			Info: volumes.VolumeInfo{
				Description: a.clusterName + "-" + volume.ID,
			},
			MountName: "scw-" + volume.ID,
			EtcdName:  "vol-" + volume.ID,
		}

		if volume.Server != nil {
			localEtcdVolume.AttachedTo = volume.Server.ID
			if volume.Server.ID == a.server.ID {
				localEtcdVolume.LocalDevice = fmt.Sprintf("%s%s", localDevicePrefix, volume.ID)
			}
		}

		localEtcdVolumes = append(localEtcdVolumes, localEtcdVolume)
	}

	return localEtcdVolumes, nil
}

// FindMountedVolume returns the device where the volume is mounted to the running server.
func (a *ScwVolumes) FindMountedVolume(volume *volumes.Volume) (string, error) {
	device := volume.LocalDevice

	klog.V(2).Infof("Finding mounted volume %q", device)
	_, err := os.Stat(volumes.PathFor(device))
	if err == nil {
		klog.V(2).Infof("Found mounted volume %q", device)
		return device, nil
	}

	if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to find local device %q: %v", device, err)
	}

	// When not found, the interface says to return ("", nil)
	return "", nil
}

// AttachVolume attaches the specified volume to the running server and returns the mountpoint if successful.
func (a *ScwVolumes) AttachVolume(volume *volumes.Volume) error {
	for {
		volumeResp, err := a.instanceAPI.GetVolume(&instance.GetVolumeRequest{
			VolumeID: volume.ProviderID,
			Zone:     a.zone,
		})
		if err != nil || volumeResp.Volume == nil {
			return fmt.Errorf("failed to get info for volume id %q: %w", volume.ProviderID, err)
		}

		scwVolume := volumeResp.Volume
		if scwVolume.Server != nil {
			if scwVolume.Server.ID != a.server.ID {
				return fmt.Errorf("found volume %s(%s) attached to a different server: %s", scwVolume.Name, scwVolume.ID, scwVolume.Server.ID)
			}
			klog.V(2).Infof("Attached volume %s(%d) of type %s to the running server", scwVolume.Name, scwVolume.ID, scwVolume.VolumeType)
			volume.LocalDevice = fmt.Sprintf("%s%s", localDevicePrefix, scwVolume.ID)
			return nil
		}

		//err = a.instanceAPI.ServerActionAndWait(&instance.ServerActionAndWaitRequest{
		//	ServerID: a.server.ID,
		//	Zone:     a.zone,
		//	Action:   instance.ServerActionPoweroff,
		//})
		//if err != nil {
		//	return fmt.Errorf("failed to power-off server %s before attaching volume", a.server.ID)
		//}

		klog.V(2).Infof("Attaching volume %s(%d) of type %s to the running server", scwVolume.Name, scwVolume.ID, scwVolume.VolumeType)
		_, err = a.instanceAPI.AttachVolume(&instance.AttachVolumeRequest{
			Zone:     a.zone,
			ServerID: a.server.ID,
			VolumeID: scwVolume.ID,
		})
		if err != nil {
			return fmt.Errorf("failed to attach volume %s(%s): %w", scwVolume.Name, scwVolume.ID, err)
		}

		//_, err = a.instanceAPI.ServerAction(&instance.ServerActionRequest{
		//	ServerID: a.server.ID,
		//	Zone:     a.zone,
		//	Action:   instance.ServerActionPoweron,
		//})
		//if err != nil {
		//	return fmt.Errorf("failed to power server %s back on after attaching volume", a.server.ID)
		//}

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
	}
}

// MyIP returns the first private IP of the running server if successful.
func (a *ScwVolumes) MyIP() (string, error) {
	if a.server.PrivateIP == nil || *a.server.PrivateIP == "" {
		return "", fmt.Errorf("failed to find private IP of server %s: ", a.server.ID)
	}
	klog.V(2).Infof("Found first private IP of the running server: %s", a.server.PrivateIP)
	return *a.server.PrivateIP, nil
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
