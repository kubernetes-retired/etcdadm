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

package aws

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"

	"kope.io/etcd-manager/pkg/volumes"
)

var devices = []string{"/dev/xvdu", "/dev/xvdv", "/dev/xvdx", "/dev/xvdx", "/dev/xvdy", "/dev/xvdz"}

// AWSVolumes defines the aws volume implementation
type AWSVolumes struct {
	mutex sync.Mutex

	matchTagKeys []string
	matchTags    map[string]string
	nameTag      string
	clusterName  string

	deviceMap  map[string]string
	ec2        *ec2.EC2
	instanceID string
	localIP    string
	metadata   *ec2metadata.EC2Metadata
	zone       string

	// endpointFormat is the format string to transform an address into a discovery endpoint
	endpointFormat string
}

var _ volumes.Volumes = &AWSVolumes{}

// NewAWSVolumes returns a new aws volume provider
func NewAWSVolumes(clusterName string, volumeTags []string, nameTag string, endpointFormat string) (*AWSVolumes, error) {
	a := &AWSVolumes{
		clusterName:    clusterName,
		deviceMap:      make(map[string]string),
		matchTags:      make(map[string]string),
		nameTag:        nameTag,
		endpointFormat: endpointFormat,
	}

	for _, volumeTag := range volumeTags {
		tokens := strings.SplitN(volumeTag, "=", 2)
		if len(tokens) == 1 {
			a.matchTagKeys = append(a.matchTagKeys, tokens[0])
		} else {
			a.matchTags[tokens[0]] = tokens[1]
		}
	}

	config := aws.NewConfig()
	config = config.WithCredentialsChainVerboseErrors(true)

	s, err := session.NewSession(config)
	if err != nil {
		return nil, fmt.Errorf("error starting new AWS session: %v", err)
	}
	s.Handlers.Send.PushFront(func(r *request.Request) {
		// Log requests
		glog.V(4).Infof("AWS API Request: %s/%s", r.ClientInfo.ServiceName, r.Operation.Name)
	})

	a.metadata = ec2metadata.New(s, config)

	region, err := a.metadata.Region()
	if err != nil {
		return nil, fmt.Errorf("error querying ec2 metadata service (for az/region): %v", err)
	}

	a.zone, err = a.metadata.GetMetadata("placement/availability-zone")
	if err != nil {
		return nil, fmt.Errorf("error querying ec2 metadata service (for az): %v", err)
	}

	a.instanceID, err = a.metadata.GetMetadata("instance-id")
	if err != nil {
		return nil, fmt.Errorf("error querying ec2 metadata service (for instance-id): %v", err)
	}

	a.localIP, err = a.metadata.GetMetadata("local-ipv4")
	if err != nil {
		return nil, fmt.Errorf("error querying ec2 metadata service (for local-ipv4): %v", err)
	}

	a.ec2 = ec2.New(s, config.WithRegion(region))

	return a, nil
}

func (a *AWSVolumes) describeInstance() (*ec2.Instance, error) {
	request := &ec2.DescribeInstancesInput{}
	request.InstanceIds = []*string{&a.instanceID}

	var instances []*ec2.Instance
	err := a.ec2.DescribeInstancesPages(request, func(p *ec2.DescribeInstancesOutput, lastPage bool) (shouldContinue bool) {
		for _, r := range p.Reservations {
			instances = append(instances, r.Instances...)
		}
		return true
	})
	if err != nil {
		return nil, fmt.Errorf("error querying for EC2 instance %q: %v", a.instanceID, err)
	}

	if len(instances) != 1 {
		return nil, fmt.Errorf("unexpected number of instances found with id %q: %d", a.instanceID, len(instances))
	}

	return instances[0], nil
}

func newEc2Filter(name string, value string) *ec2.Filter {
	filter := &ec2.Filter{
		Name: aws.String(name),
		Values: []*string{
			aws.String(value),
		},
	}
	return filter
}

func (a *AWSVolumes) describeVolumes(request *ec2.DescribeVolumesInput) ([]*volumes.Volume, error) {
	var found []*volumes.Volume
	err := a.ec2.DescribeVolumesPages(request, func(p *ec2.DescribeVolumesOutput, lastPage bool) (shouldContinue bool) {
		for _, v := range p.Volumes {
			etcdName := aws.StringValue(v.VolumeId)
			if a.nameTag != "" {
				for _, t := range v.Tags {
					if a.nameTag == aws.StringValue(t.Key) {
						v := aws.StringValue(t.Value)
						if v != "" {
							tokens := strings.SplitN(v, "/", 2)
							etcdName = a.clusterName + "-" + tokens[0]
						}
					}
				}
			}

			vol := &volumes.Volume{
				ProviderID: aws.StringValue(v.VolumeId),
				EtcdName:   etcdName,
				Info: volumes.VolumeInfo{
					Description: aws.StringValue(v.VolumeId),
				},
			}
			state := aws.StringValue(v.State)

			vol.Status = state

			for _, attachment := range v.Attachments {
				vol.AttachedTo = aws.StringValue(attachment.InstanceId)
				if aws.StringValue(attachment.InstanceId) == a.instanceID {
					vol.LocalDevice = aws.StringValue(attachment.Device)
				}
			}

			// never mount root volumes
			// these are volumes that aws sets aside for root volumes mount points
			if vol.LocalDevice == "/dev/sda1" || vol.LocalDevice == "/dev/xvda" {
				glog.Warningf("Not mounting: %q, since it is a root volume", vol.LocalDevice)
				continue
			}

			found = append(found, vol)
		}
		return true
	})

	if err != nil {
		return nil, fmt.Errorf("error querying for EC2 volumes: %v", err)
	}
	return found, nil
}

func (a *AWSVolumes) FindVolumes() ([]*volumes.Volume, error) {
	return a.findVolumes(true)
}

func (a *AWSVolumes) findVolumes(filterByAZ bool) ([]*volumes.Volume, error) {
	request := &ec2.DescribeVolumesInput{}

	if filterByAZ {
		request.Filters = append(request.Filters, newEc2Filter("availability-zone", a.zone))
	}

	for k, v := range a.matchTags {
		request.Filters = append(request.Filters, newEc2Filter("tag:"+k, v))
	}
	for _, k := range a.matchTagKeys {
		request.Filters = append(request.Filters, newEc2Filter("tag-key", k))
	}

	return a.describeVolumes(request)
}

// FindMountedVolume implements Volumes::FindMountedVolume
func (a *AWSVolumes) FindMountedVolume(volume *volumes.Volume) (string, error) {
	device := volume.LocalDevice

	_, err := os.Stat(volumes.PathFor(device))
	if err == nil {
		return device, nil
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("error checking for device %q: %v", device, err)
	}
	glog.V(2).Infof("volume %s not mounted at %s", volume.ProviderID, volumes.PathFor(device))

	if volume.ProviderID != "" {
		expected := volume.ProviderID
		expected = "nvme-Amazon_Elastic_Block_Store_" + strings.Replace(expected, "-", "", -1)

		// Look for nvme devices
		// On AWS, nvme volumes are not mounted on a device path, but are instead mounted on an nvme device
		// We must identify the correct volume by matching the nvme info
		device, err := findNvmeVolume(expected)
		if err != nil {
			return "", fmt.Errorf("error checking for nvme volume %q: %v", expected, err)
		}
		if device != "" {
			glog.Infof("found nvme volume %q at %q", expected, device)
			return device, nil
		}
		glog.V(2).Infof("volume %s not mounted at %s", volume.ProviderID, expected)
	}

	return "", nil
}

func findNvmeVolume(findName string) (device string, err error) {
	p := volumes.PathFor(filepath.Join("/dev/disk/by-id", findName))
	stat, err := os.Lstat(p)
	if err != nil {
		if os.IsNotExist(err) {
			glog.V(4).Infof("nvme path not found %q", p)
			return "", nil
		}
		return "", fmt.Errorf("error getting stat of %q: %v", p, err)
	}

	if stat.Mode()&os.ModeSymlink != os.ModeSymlink {
		glog.Warningf("nvme file %q found, but was not a symlink", p)
		return "", nil
	}

	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", fmt.Errorf("error reading target of symlink %q: %v", p, err)
	}

	// Reverse pathFor
	devPath := volumes.PathFor("/dev")
	if strings.HasPrefix(resolved, devPath) {
		resolved = strings.Replace(resolved, devPath, "/dev", 1)
	}

	if !strings.HasPrefix(resolved, "/dev") {
		return "", fmt.Errorf("resolved symlink for %q was unexpected: %q", p, resolved)
	}

	return resolved, nil
}

// assignDevice picks a hopefully unused device and reserves it for the volume attachment
func (a *AWSVolumes) assignDevice(volumeID string) (string, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// TODO: Check for actual devices in use (like cloudprovider does)
	for _, d := range devices {
		if a.deviceMap[d] == "" {
			a.deviceMap[d] = volumeID
			return d, nil
		}
	}
	return "", fmt.Errorf("All devices in use")
}

// releaseDevice releases the volume mapping lock; used when an attach was known to fail
func (a *AWSVolumes) releaseDevice(d string, volumeID string) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if a.deviceMap[d] != volumeID {
		glog.Fatalf("deviceMap logic error: %q -> %q, not %q", d, a.deviceMap[d], volumeID)
	}
	a.deviceMap[d] = ""
}

// AttachVolume attaches the specified volume to this instance, returning the mountpoint & nil if successful
func (a *AWSVolumes) AttachVolume(volume *volumes.Volume) error {
	volumeID := volume.ProviderID

	device := volume.LocalDevice
	if device == "" {
		d, err := a.assignDevice(volumeID)
		if err != nil {
			return err
		}
		device = d

		request := &ec2.AttachVolumeInput{
			Device:     aws.String(device),
			InstanceId: aws.String(a.instanceID),
			VolumeId:   aws.String(volumeID),
		}

		attachResponse, err := a.ec2.AttachVolume(request)
		if err != nil {
			return fmt.Errorf("Error attaching EBS volume %q: %v", volumeID, err)
		}

		glog.V(2).Infof("AttachVolume request returned %v", attachResponse)
	}

	// Wait (forever) for volume to attach or reach a failure-to-attach condition
	for {
		request := &ec2.DescribeVolumesInput{
			VolumeIds: []*string{&volumeID},
		}

		volumes, err := a.describeVolumes(request)
		if err != nil {
			return fmt.Errorf("Error describing EBS volume %q: %v", volumeID, err)
		}

		if len(volumes) == 0 {
			return fmt.Errorf("EBS volume %q disappeared during attach", volumeID)
		}
		if len(volumes) != 1 {
			return fmt.Errorf("Multiple volumes found with id %q", volumeID)
		}

		v := volumes[0]
		if v.AttachedTo != "" {
			if v.AttachedTo == a.instanceID {
				// TODO: Wait for device to appear?

				volume.LocalDevice = device
				return nil
			} else {
				a.releaseDevice(device, volumeID)

				return fmt.Errorf("Unable to attach volume %q, was attached to %q", volumeID, v.AttachedTo)
			}
		}

		switch v.Status {
		case "attaching":
			glog.V(2).Infof("Waiting for volume %q to be attached (currently %q)", volumeID, v.Status)
			// continue looping

		default:
			return fmt.Errorf("Observed unexpected volume state %q", v.Status)
		}

		time.Sleep(10 * time.Second)
	}
}

func (a *AWSVolumes) MyIP() (string, error) {
	return a.localIP, nil
}
