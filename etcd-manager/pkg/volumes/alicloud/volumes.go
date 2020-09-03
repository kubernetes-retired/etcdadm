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

package alicloud

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	alicloud "github.com/aliyun/alibaba-cloud-sdk-go/sdk"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/auth/credentials"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"k8s.io/klog"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/volumes"
)

const (
	// We could query at most 50 disks at a time on Alicloud
	PageSizeLarge = 50
)

type AlicloudVolumes struct {
	mutex sync.Mutex

	matchTagKeys []string
	matchTags    map[string]string
	nameTag      string
	clusterName  string

	deviceMap  map[string]string
	ecs        *ecs.Client
	instanceID string
	localIP    string
	metadata   *ECSMetadata
	az         string
	region     string
}

var _ volumes.Volumes = &AlicloudVolumes{}

// NewAlicloudVolumes returns a new aws volume provider
func NewAlicloudVolumes(clusterName string, volumeTags []string, nameTag string) (*AlicloudVolumes, error) {
	a := &AlicloudVolumes{
		clusterName: clusterName,
		deviceMap:   make(map[string]string),
		matchTags:   make(map[string]string),
		nameTag:     nameTag,
	}

	for _, volumeTag := range volumeTags {
		tokens := strings.SplitN(volumeTag, "=", 2)
		if len(tokens) == 1 {
			a.matchTagKeys = append(a.matchTagKeys, tokens[0])
		} else {
			a.matchTags[tokens[0]] = tokens[1]
		}
	}

	a.metadata = NewECSMetadata()

	var err error

	a.region, err = a.metadata.GetMetadata("region-id")
	if err != nil {
		return nil, fmt.Errorf("error querying ecs metadata (for region): %v", err)
	}

	a.az, err = a.metadata.GetMetadata("zone-id")
	if err != nil {
		return nil, fmt.Errorf("error querying ecs metadata (for az): %v", err)
	}

	a.instanceID, err = a.metadata.GetMetadata("instance-id")
	if err != nil {
		return nil, fmt.Errorf("error querying ecs metadata (for instance-id): %v", err)
	}

	a.localIP, err = a.metadata.GetMetadata("private-ipv4")
	if err != nil {
		return nil, fmt.Errorf("error querying ecs metadata (for private-ipv4): %v", err)
	}

	ramRole, err := a.metadata.GetMetadata("ram/security-credentials/")
	if err != nil {
		return nil, fmt.Errorf("error querying ecs metadata (for RAM role): %v", err)
	}

	config := alicloud.NewConfig()

	credential := credentials.NewEcsRamRoleCredential(ramRole)
	a.ecs, err = ecs.NewClientWithOptions(a.region, config, credential)
	if err != nil {
		return nil, fmt.Errorf("error creating ecs sdk client: %v", err)
	}

	return a, nil
}

// AttachVolume attaches the specified volume to this instance, returning the mountpoint & nil if successful
func (a *AlicloudVolumes) AttachVolume(volume *volumes.Volume) error {
	volumeID := volume.ProviderID

	if volume.LocalDevice == "" {
		attachDiskReq := ecs.CreateAttachDiskRequest()
		attachDiskReq.RegionId = a.region
		attachDiskReq.InstanceId = a.instanceID
		attachDiskReq.DiskId = volumeID

		attachResponse, err := a.ecs.AttachDisk(attachDiskReq)
		if err != nil {
			return fmt.Errorf("Unable to attach volume %q: %v", volumeID, err)
		}

		klog.V(2).Infof("AttachVolume request returned %v", attachResponse)

		// Attaching volume takes time.
		time.Sleep(5)
	}

	// Set a 5 min timeout for this loop
	for i := 0; i < 30; i++ {
		request := ecs.CreateDescribeDisksRequest()
		request.PageSize = requests.NewInteger(PageSizeLarge)
		request.PageNumber = requests.NewInteger(1)
		request.DiskIds = arrayString([]string{volumeID})

		volumes, err := a.describeVolumes(request)
		if err != nil {
			return fmt.Errorf("Error describing volume %q: %v", volumeID, err)
		}

		if len(volumes) == 0 {
			return fmt.Errorf("Disk volume %q disappeared during attaching", volumeID)
		}
		if len(volumes) != 1 {
			return fmt.Errorf("Multiple volumes found with id: %q", volumeID)
		}

		v := volumes[0]
		if v.AttachedTo != "" {
			if v.AttachedTo == a.instanceID {
				volume.LocalDevice = v.LocalDevice
				return nil
			}

			return fmt.Errorf("Unable to attach volume %q, was attached to %q", volumeID, v.AttachedTo)
		}

		switch v.Status {
		case "Attaching":
			klog.V(2).Infof("Waiting for volume %q to be attached (currently %q)", volumeID, v.Status)
			// continue looping

		default:
			return fmt.Errorf("Observed unexpected volume state %q", v.Status)
		}

		time.Sleep(10 * time.Second)
	}

	return fmt.Errorf("fail to attach disk: %q in 5 mins", volumeID)
}

func (a *AlicloudVolumes) FindVolumes() ([]*volumes.Volume, error) {
	return a.findVolumes(true)
}

func (a *AlicloudVolumes) findVolumes(filterByAZ bool) ([]*volumes.Volume, error) {
	request := ecs.CreateDescribeDisksRequest()
	request.RegionId = a.region

	if filterByAZ {
		request.ZoneId = a.az
	}

	var tags []ecs.DescribeDisksTag

	for _, key := range a.matchTagKeys {
		tags = append(tags, ecs.DescribeDisksTag{
			Key:   key,
			Value: "",
		})
	}

	for key, value := range a.matchTags {
		tags = append(tags, ecs.DescribeDisksTag{
			Key:   key,
			Value: value,
		})
	}

	request.Tag = &tags
	request.PageSize = requests.NewInteger(PageSizeLarge)
	request.PageNumber = requests.NewInteger(1)

	return a.describeVolumes(request)
}

func (a *AlicloudVolumes) describeVolumes(request *ecs.DescribeDisksRequest) ([]*volumes.Volume, error) {
	var found []*volumes.Volume

	for {
		klog.V(4).Infof("describing Alicloud volumes page %v", request.PageNumber)
		response, err := a.ecs.DescribeDisks(request)
		if err != nil {
			return nil, err
		}

		if response == nil || len(response.Disks.Disk) < 1 {
			break
		}

		disks := response.Disks.Disk

		for _, disk := range disks {
			etcdName := disk.DiskId

			if a.nameTag != "" {
				for _, t := range disk.Tags.Tag {
					if a.nameTag == t.TagKey {
						v := t.TagValue
						if v != "" {
							tokens := strings.SplitN(v, "/", 2)
							etcdName = a.clusterName + "-" + tokens[0]
						}
					}
				}
			}

			vol := &volumes.Volume{
				MountName:  "master-" + disk.DiskId,
				ProviderID: disk.DiskId,
				EtcdName:   etcdName,
				Info: volumes.VolumeInfo{
					Description: disk.Description,
				},
				Status:     disk.Status,
				AttachedTo: disk.InstanceId,
			}

			if vol.AttachedTo == a.instanceID {
				vol.LocalDevice = disk.Device
			}

			found = append(found, vol)
		}

		if len(response.Disks.Disk) < PageSizeLarge {
			break
		}

		nextPage, err := getNextpageNumber(request.PageNumber)
		if err != nil {
			return nil, err
		}
		request.PageNumber = nextPage
	}

	return found, nil
}

func (a *AlicloudVolumes) MyIP() (string, error) {
	return a.localIP, nil
}

func getNextpageNumber(number requests.Integer) (requests.Integer, error) {
	page, err := strconv.Atoi(string(number))
	if err != nil {
		return "", err
	}
	return requests.NewInteger(page + 1), nil
}

// FindMountedVolume implements Volumes::FindMountedVolume
func (a *AlicloudVolumes) FindMountedVolume(volume *volumes.Volume) (string, error) {
	device := volume.LocalDevice

	_, err := os.Stat(volumes.PathFor(device))
	if err == nil {
		return device, nil
	}
	if os.IsNotExist(err) {
		// When attaching a disk as /dev/xvdb in web console, it is attached as /dev/vdb in OS,
		// which is very strange. Have no idea why Alicloud does this strange thing.
		if strings.HasPrefix(device, "/dev/xvd") {
			device = "/dev/vd" + strings.TrimPrefix(device, "/dev/xvd")
			_, err = os.Stat(volumes.PathFor(device))
			return device, err
		}
		if strings.HasPrefix(device, "/dev/vd") {
			device = "/dev/xvd" + strings.TrimPrefix(device, "/dev/vd")
			_, err = os.Stat(volumes.PathFor(device))
			return device, err
		}
		return "", nil
	}
	return "", fmt.Errorf("error checking for device %q: %v", device, err)
}

// Some of Alicloud APIs take array string as arguments.
func arrayString(a []string) string {
	return "[\"" + strings.Join(a, "\",\"") + "\"]"
}
