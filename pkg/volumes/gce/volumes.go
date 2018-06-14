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

package gce

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"

	"cloud.google.com/go/compute/metadata"
	"github.com/golang/glog"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v0.beta"
	"kope.io/etcd-manager/pkg/volumes"
)

// GCEVolumes defines the aws volume implementation
type GCEVolumes struct {
	mutex sync.Mutex

	matchTagKeys []string
	matchTags    map[string]string
	nameTag      string
	clusterName  string

	compute *compute.Service

	project string
	zone    string
	region  string

	instanceName string
	internalIP   net.IP

	allZonesInRegion []string

	// endpointFormat is the format string to transform an address into a discovery endpoint
	endpointFormat string
}

var _ volumes.Volumes = &GCEVolumes{}

// NewGCEVolumes returns a new aws volume provider
func NewGCEVolumes(clusterName string, volumeTags []string, nameTag string, endpointFormat string) (*GCEVolumes, error) {
	g := &GCEVolumes{
		clusterName:    clusterName,
		matchTags:      make(map[string]string),
		nameTag:        nameTag,
		endpointFormat: endpointFormat,
	}

	for _, volumeTag := range volumeTags {
		tokens := strings.SplitN(volumeTag, "=", 2)
		if len(tokens) == 1 {
			g.matchTagKeys = append(g.matchTagKeys, tokens[0])
		} else {
			g.matchTags[tokens[0]] = tokens[1]
		}
	}

	ctx := context.Background()

	client, err := google.DefaultClient(ctx, compute.ComputeScope)
	if err != nil {
		return nil, fmt.Errorf("error building google API client: %v", err)
	}
	computeService, err := compute.New(client)
	if err != nil {
		return nil, fmt.Errorf("error building compute API client: %v", err)
	}
	g.compute = computeService

	// Project ID
	{
		project, err := metadata.ProjectID()
		if err != nil {
			return nil, fmt.Errorf("error reading project from GCE: %v", err)
		}
		g.project = strings.TrimSpace(project)
		if g.project == "" {
			return nil, fmt.Errorf("project metadata was empty")
		}
		glog.Infof("Found project=%q", g.project)
	}

	// Zone
	{
		zone, err := metadata.Zone()
		if err != nil {
			return nil, fmt.Errorf("error reading zone from GCE: %v", err)
		}
		g.zone = strings.TrimSpace(zone)
		if g.zone == "" {
			return nil, fmt.Errorf("zone metadata was empty")
		}
		glog.Infof("Found zone=%q", g.zone)

		region, err := regionFromZone(zone)
		if err != nil {
			return nil, fmt.Errorf("error determining region from zone %q: %v", zone, err)
		}
		g.region = region
		glog.Infof("Found region=%q", g.region)
	}

	// Instance Name
	{
		instanceName, err := metadata.InstanceName()
		if err != nil {
			return nil, fmt.Errorf("error reading instance name from GCE: %v", err)
		}
		g.instanceName = strings.TrimSpace(instanceName)
		if g.instanceName == "" {
			return nil, fmt.Errorf("instance name metadata was empty")
		}
		glog.Infof("Found instanceName=%q", g.instanceName)
	}

	// Internal IP
	{
		internalIP, err := metadata.InternalIP()
		if err != nil {
			return nil, fmt.Errorf("error querying InternalIP from GCE: %v", err)
		}
		if internalIP == "" {
			return nil, fmt.Errorf("InternalIP from metadata was empty")
		}
		g.internalIP = net.ParseIP(internalIP)
		if g.internalIP == nil {
			return nil, fmt.Errorf("InternalIP from metadata was not parseable(%q)", internalIP)
		}
		glog.Infof("Found internalIP=%q", g.internalIP)
	}

	{
		zones, err := computeService.Zones.List(g.project).Do()
		if err != nil {
			return nil, fmt.Errorf("error querying for GCE zones: %v", err)
		}

		var allZonesInRegion []string
		for _, z := range zones.Items {
			regionName := lastComponent(z.Region)
			if regionName != g.region {
				continue
			}
			allZonesInRegion = append(allZonesInRegion, z.Name)
		}
		g.allZonesInRegion = allZonesInRegion
	}

	return g, nil
}

func (g *GCEVolumes) matchesTags(d *compute.Disk) bool {
	for _, k := range g.matchTagKeys {
		_, found := d.Labels[k]
		if !found {
			return false
		}
	}

	for k, v := range g.matchTags {
		a, found := d.Labels[k]
		if !found || a != v {
			return false
		}
	}

	return true
}

// DecodeGCELabel reverse EncodeGCELabel, taking the encoded RFC1035 compatible value back to a string
func DecodeGCELabel(s string) (string, error) {
	uriForm := strings.Replace(s, "-", "%", -1)
	v, err := url.QueryUnescape(uriForm)
	if err != nil {
		return "", fmt.Errorf("Cannot decode GCE label: %q", s)
	}
	return v, nil
}

func (g *GCEVolumes) buildGCEVolume(d *compute.Disk) (*volumes.Volume, error) {
	etcdName := d.Name
	if g.nameTag != "" {
		v := d.Labels[g.nameTag]
		if v != "" {
			plaintext, err := DecodeGCELabel(v)
			if err != nil {
				return nil, fmt.Errorf("Error decoding GCE label: %s=%q", g.nameTag, v)
			}

			tokens := strings.SplitN(plaintext, "/", 2)
			etcdName = g.clusterName + "-" + tokens[0]
		}
	}

	vol := &volumes.Volume{
		ProviderID: d.SelfLink,
		EtcdName:   etcdName,
		Info: volumes.VolumeInfo{
			Description: d.SelfLink,
		},
	}

	vol.Status = d.Status

	for _, attachedTo := range d.Users {
		u, err := ParseGoogleCloudURL(attachedTo)
		if err != nil {
			return nil, fmt.Errorf("error parsing disk attachmnet url %q: %v", attachedTo, err)
		}

		vol.AttachedTo = attachedTo

		if u.Project == g.project && u.Zone == g.zone && u.Name == g.instanceName {
			devicePath := "/dev/disk/by-id/google-" + d.Name
			vol.LocalDevice = devicePath
			glog.V(2).Infof("volume %q is attached to this instance at %s", d.Name, devicePath)
		} else {
			glog.V(2).Infof("volume %q is attached to another instance %q", d.Name, attachedTo)
		}
	}

	return vol, nil
}

func (g *GCEVolumes) FindVolumes() ([]*volumes.Volume, error) {
	return g.findVolumes(true)
}

func (g *GCEVolumes) findVolumes(filterByZone bool) ([]*volumes.Volume, error) {
	var volumes []*volumes.Volume

	glog.V(2).Infof("Listing GCE disks in %s/%s", g.project, g.zone)

	var zones []string

	if filterByZone {
		zones = []string{g.zone}
	} else {
		zones = g.allZonesInRegion
	}

	// TODO: Apply filters
	ctx := context.Background()
	for _, zone := range zones {
		err := g.compute.Disks.List(g.project, zone).Pages(ctx, func(page *compute.DiskList) error {
			for _, d := range page.Items {
				if !g.matchesTags(d) {
					continue
				}

				vol, err := g.buildGCEVolume(d)
				if err != nil {
					// Fail safe
					glog.Warningf("skipping malformed volume %q: %v", d.Name, err)
					continue
				}
				volumes = append(volumes, vol)
			}

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("error querying GCE disks: %v", err)
		}
	}

	return volumes, nil
}

// AttachVolume attaches the specified volume to this instance, returning the mountpoint & nil if successful
func (g *GCEVolumes) AttachVolume(volume *volumes.Volume) error {
	volumeURL := volume.ProviderID

	volumeName := lastComponent(volumeURL)

	attachedDisk := &compute.AttachedDisk{
		DeviceName: volumeName,
		// TODO: The k8s GCE provider sets Kind, but this seems wrong.  Open an issue?
		//Kind:       disk.Kind,
		Mode:   "READ_WRITE",
		Source: volumeURL,
		Type:   "PERSISTENT",
	}

	attachOp, err := g.compute.Instances.AttachDisk(g.project, g.zone, g.instanceName, attachedDisk).Do()
	if err != nil {
		return fmt.Errorf("error attaching disk %q: %v", volumeName, err)
	}

	err = WaitForOp(g.compute, attachOp)
	if err != nil {
		return fmt.Errorf("error waiting for disk attach to complete %q: %v", volumeName, err)
	}

	devicePath := "/dev/disk/by-id/google-" + volumeName

	// TODO: Wait for device to appear?

	volume.LocalDevice = devicePath

	return nil
}

// FindMountedVolume implements Volumes::FindMountedVolume
func (g *GCEVolumes) FindMountedVolume(volume *volumes.Volume) (string, error) {
	device := volume.LocalDevice

	_, err := os.Stat(volumes.PathFor(device))
	if err == nil {
		return device, nil
	}
	if os.IsNotExist(err) {
		return "", nil
	}
	return "", fmt.Errorf("error checking for device %q: %v", device, err)
}

func (g *GCEVolumes) MyIP() (string, error) {
	if g.internalIP == nil {
		return "", fmt.Errorf("unable to determine local IP")
	}
	return g.internalIP.String(), nil
}
