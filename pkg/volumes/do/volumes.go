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
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/digitalocean/godo"
	"github.com/golang/glog"

	"kope.io/etcd-manager/pkg/volumes"
)

// DigiCloud exposes all the interfaces required to operate on DigitalOcean resources
type DigiCloud struct {
	Client *godo.Client
	Region string
}

const (
	dropletRegionMetadataURL     = "http://169.254.169.254/metadata/v1/region"
	dropletNameMetadataURL       = "http://169.254.169.254/metadata/v1/hostname"
	dropletIDMetadataURL         = "http://169.254.169.254/metadata/v1/id"
	dropletIDMetadataTags        = "http://169.254.169.254/metadata/v1/tags"
	dropletInternalIPMetadataURL = "http://169.254.169.254/metadata/v1/interfaces/private/0/ipv4/address"
	localDevicePrefix            = "/dev/disk/by-id/scsi-0DO_Volume_"
)

// TokenSource implements oauth2.TokenSource
type TokenSource struct {
	AccessToken string
}

// Token() returns oauth2.Token
func (t *TokenSource) Token() (*oauth2.Token, error) {
	token := &oauth2.Token{
		AccessToken: t.AccessToken,
	}
	return token, nil
}

// DOVolumes defines the digital ocean's volume implementation
type DOVolumes struct {
	ClusterName  string
	DigiCloud    *DigiCloud
	region       string
	dropletName  string
	dropletID    int
	nameTag      string
	matchTagKeys []string
	matchTags    map[string]string
	dropletTags  []string
}

var _ volumes.Volumes = &DOVolumes{}

// ex: nametag - etcdcluster-main OR etcdcluster-events
// ex: volume tag array - kubernetescluster=mycluster; k8s-index
// any droplet where this is running will have the tags - k8s-index:1; kubernetescluster:mycluster
// any volume that is created will have the tags - etcdcluster-main:1 OR etcdcluster-events:1; kubernetescluster:mycluster; k8s-index:1
func NewDOVolumes(clusterName string, volumeTags []string, nameTag string) (*DOVolumes, error) {
	region, err := getMetadataRegion()
	if err != nil {
		return nil, fmt.Errorf("failed to get droplet region: %s", err)
	}

	dropletID, err := getMetadataDropletID()
	if err != nil {
		return nil, fmt.Errorf("failed to get droplet id: %s", err)
	}

	dropletIDInt, err := strconv.Atoi(dropletID)
	if err != nil {
		return nil, fmt.Errorf("failed to convert droplet ID to int: %s", err)
	}

	dropletName, err := getMetadataDropletName()
	if err != nil {
		return nil, fmt.Errorf("failed to get droplet name: %s", err)
	}

	dropletTags, err := getMetadataDropletTags()
	if err != nil {
		return nil, fmt.Errorf("failed to get droplet tags: %s", err)
	}

	cloud, err := NewCloud(region)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize digitalocean cloud: %s", err)
	}

	a := &DOVolumes{
		DigiCloud:   cloud,
		ClusterName: clusterName,
		dropletID:   dropletIDInt,
		dropletName: dropletName,
		region:      region,
		nameTag:     nameTag,
		matchTags:   make(map[string]string),
		dropletTags: dropletTags,
	}

	for _, volumeTag := range volumeTags {
		tokens := strings.SplitN(volumeTag, "=", 2)
		if len(tokens) == 1 {
			a.matchTagKeys = append(a.matchTagKeys, tokens[0])
		} else {
			a.matchTags[tokens[0]] = tokens[1]
		}
	}

	return a, nil
}

func (a *DOVolumes) FindVolumes() ([]*volumes.Volume, error) {
	return a.findVolumes(true)
}

func (a *DOVolumes) findAllVolumes(filterByRegion bool) ([]*volumes.Volume, error) {
	doVolumes, err := getAllVolumesByRegion(a.DigiCloud, a.region, filterByRegion)
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes: %s", err)
	}

	var myvolumes []*volumes.Volume
	for _, doVolume := range doVolumes {

		glog.V(2).Infof("Iterating DO Volume with name=%s ID=%s, nametag=%s", doVolume.Name, doVolume.ID, a.nameTag)

		// make sure dropletTags match the volumeTags for the keys mentioned in matchTags
		tagFound := a.matchDropletTags(&doVolume)

		if tagFound {

			glog.V(2).Infof("Tag Matched for droplet name=%s and volume name=%s", a.dropletName, doVolume.Name)

			var clusterKey string
			if strings.Contains(doVolume.Name, "etcd-main") {
				clusterKey = "main"
			} else if strings.Contains(doVolume.Name, "etcd-events") {
				clusterKey = "events"
			} else {
				glog.V(2).Infof("could not determine etcd cluster type for volume: %s", doVolume.Name)
			}

			if strings.Contains(a.nameTag, clusterKey) {
				vol := &volumes.Volume{
					ProviderID: doVolume.ID,
					Info: volumes.VolumeInfo{
						Description: a.ClusterName + "-" + a.nameTag,
					},
					MountName: "master-" + doVolume.ID,
					EtcdName:  doVolume.Name,
				}

				if len(doVolume.DropletIDs) > 0 {
					vol.AttachedTo = strconv.Itoa(doVolume.DropletIDs[0])
					vol.LocalDevice = getLocalDeviceName(&doVolume)
				}

				glog.V(2).Infof("Found a matching nameTag=%s with etcd cluster name = %s; volume name = %s", a.nameTag, a.ClusterName, doVolume.Name)

				myvolumes = append(myvolumes, vol)
			}
		}
	}

	return myvolumes, nil
}

// ex: nametag - etcdcluster-main OR etcdcluster-events
// volumetag array - kubernetescluster=mycluster; k8s-index
// any droplet where this is running will have the tags - k8s-index:1; kubernetescluster:mycluster
// any volume that is created will have the tags - kubernetescluster:mycluster; k8s-index:1
// matchTags for kubernetescluster=clustername
// matchTagKeys for k8s-index - ensure the value is same in droplets and the volume tags.
// also check for nameTag if it is contained in the volume name (for etcd-main or etcd-events)
func (a *DOVolumes) findVolumes(filterByRegion bool) ([]*volumes.Volume, error) {
	doVolumes, err := getAllVolumesByRegion(a.DigiCloud, a.region, filterByRegion)
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes: %s", err)
	}

	var myvolumes []*volumes.Volume
	for _, doVolume := range doVolumes {

		glog.V(2).Infof("DO Volume name=%s, Volume ID=%s, nametag=%s", doVolume.Name, doVolume.ID, a.nameTag)

		glog.V(2).Infof("Iterating all Volume tags for volume %q,  tags %v, droplet tags %v", doVolume.Name, doVolume.Tags, a.dropletTags)

		// make sure dropletTags match the volumeTags for the keys mentioned in matchTags
		tagFound := a.matchesTags(&doVolume)

		if tagFound {

			glog.V(2).Infof("Tag Matched for droplet name=%s and volume name=%s", a.dropletName, doVolume.Name)

			var clusterKey string
			if strings.Contains(doVolume.Name, "etcd-main") {
				clusterKey = "main"
			} else if strings.Contains(doVolume.Name, "etcd-events") {
				clusterKey = "events"
			} else {
				glog.V(2).Infof("could not determine etcd cluster type for volume: %s", doVolume.Name)
			}

			if strings.Contains(a.nameTag, clusterKey) {
				vol := &volumes.Volume{
					ProviderID: doVolume.ID,
					Info: volumes.VolumeInfo{
						Description: a.ClusterName + "-" + a.nameTag,
					},
					MountName: "master-" + doVolume.ID,
					EtcdName:  doVolume.Name,
				}

				if len(doVolume.DropletIDs) > 0 {
					vol.AttachedTo = strconv.Itoa(doVolume.DropletIDs[0])
					vol.LocalDevice = getLocalDeviceName(&doVolume)
				}

				glog.V(2).Infof("Found a matching nameTag=%s for cluster key=%s with etcd cluster name = %s; volume name = %s", a.nameTag, clusterKey, a.ClusterName, doVolume.Name)

				myvolumes = append(myvolumes, vol)
			}
		}
	}

	return myvolumes, nil
}

func (a *DOVolumes) matchDropletTags(volume *godo.Volume) bool {
	for k, v := range a.matchTags {
		// In DO, we don't have tags with key value pairs. So we are storing them separating by a ":"
		// Create a string like "k:v" and check if they match.
		doTag := k + ":" + v

		glog.V(2).Infof("Check for matching volume tag - doTag=%s for volume name = %s", strings.ToUpper(doTag), volume.Name)

		volumeTagfound := a.Contains(volume.Tags, doTag)
		if !volumeTagfound {
			return false
		} else {
			glog.V(2).Infof("Matching tag found for doTag=%s in volume name = %s", strings.ToUpper(doTag), volume.Name)
		}

		glog.V(2).Infof("Check for matching droplet tag - doTag=%s for droplet name = %s", strings.ToUpper(doTag), a.dropletName)

		// Also check if this tag is seen in the droplet.
		dropletTagfound := a.Contains(a.dropletTags, doTag)
		if !dropletTagfound {
			return false
		} else {
			glog.V(2).Infof("Matching tag found for doTag=%s in droplet name = %s", strings.ToUpper(doTag), a.dropletName)
		}
	}

	return true
}

func (a *DOVolumes) matchesTags(volume *godo.Volume) bool {

	tagFound := a.matchDropletTags(volume)

	// Check if match is Found, otherwise, just return if nothing found.
	if !tagFound {
		return false
	}

	matchingTagCount := 0
	// now that the tags are matched, check tagkeys if it exists in both volume and droplet tags, and ensure the key values in the volume and droplet tags match.
	for _, matchTag := range a.matchTagKeys {
		for _, volumeTag := range volume.Tags {
			vt := strings.SplitN(volumeTag, ":", -1)
			if len(vt) < 1 {
				// not interested in this tag.
				continue
			}

			vtKey := vt[0]
			vtValue := vt[1]

			glog.V(2).Infof("Matching tag keys for vtKey=%s; vtValue = %s; matchTagKey = %s", strings.ToUpper(vtKey), strings.ToUpper(vtValue), strings.ToUpper(matchTag))

			if strings.ToUpper(matchTag) == strings.ToUpper(vtKey) {
				glog.V(2).Infof("Match found for tag keys for vtKey=%s; vtValue = %s; matchTagKey = %s", strings.ToUpper(vtKey), strings.ToUpper(vtValue), strings.ToUpper(matchTag))

				// match found, verify if this tag exists in dropletTags and find a matching value.
				for _, dropletTag := range a.dropletTags {
					dt := strings.SplitN(dropletTag, ":", -1)
					dtKey := dt[0]
					dtValue := dt[1]

					glog.V(2).Infof("Matching droplet tag keys for dtKey=%s; dtValue = %s; matchTagKey = %s", strings.ToUpper(dtKey), strings.ToUpper(dtValue), strings.ToUpper(matchTag))

					if strings.ToUpper(matchTag) == strings.ToUpper(dtKey) {
						glog.V(2).Infof("Matching droplet key FOUND for dtKey=%s; dtValue = %s; matchTagKey = %s", strings.ToUpper(dtKey), strings.ToUpper(dtValue), strings.ToUpper(matchTag))

						// droplet tag also matched, check if the value matches.
						if strings.ToUpper(dtValue) == strings.ToUpper(vtValue) {
							glog.V(2).Infof("Everything matched for matchTagKey = %s", strings.ToUpper(matchTag))

							matchingTagCount++
						}
					}
				}
			}

		}
	}

	if len(a.matchTagKeys) == matchingTagCount {
		return true
	}

	return false
}

// Contains tells whether a contains x.
func (a *DOVolumes) Contains(tags []string, x string) bool {
	for _, n := range tags {
		if strings.ToUpper(x) == strings.ToUpper(n) {
			return true
		}
	}
	return false
}

// FindMountedVolume implements Volumes::FindMountedVolume
func (a *DOVolumes) FindMountedVolume(volume *volumes.Volume) (string, error) {
	device := volume.LocalDevice

	_, err := os.Stat(volumes.PathFor(device))
	if err == nil {
		return device, nil
	}

	if !os.IsNotExist(err) {
		return "", fmt.Errorf("error checking for device %q: %v", device, err)
	}

	return "", fmt.Errorf("error checking for device %q: %v", device, err)
}

// AttachVolume attaches the specified volume to this instance, returning the mountpoint & nil if successful
func (a *DOVolumes) AttachVolume(volume *volumes.Volume) error {
	for {
		action, _, err := a.DigiCloud.Client.StorageActions.Attach(context.TODO(), volume.ProviderID, a.dropletID)
		if err != nil {
			return fmt.Errorf("error attaching volume: %s", err)
		}

		if action.Status != godo.ActionInProgress && action.Status != godo.ActionCompleted {
			return fmt.Errorf("invalid status for digitalocean volume: %s", volume.ProviderID)
		}

		doVolume, err := getVolumeByID(a.DigiCloud, volume.ProviderID)
		if err != nil {
			return fmt.Errorf("error getting volume status: %s", err)
		}

		if len(doVolume.DropletIDs) > 0 {

			if len(doVolume.DropletIDs) > 1 {
				return fmt.Errorf("multiple attachments found for volume %q", doVolume.Name)
			}

			if doVolume.DropletIDs[0] != a.dropletID {
				return fmt.Errorf("digitalocean volume %s is attached to another droplet", doVolume.ID)
			}

			volume.LocalDevice = getLocalDeviceName(doVolume)
			return nil
		}

		time.Sleep(10 * time.Second)
	}
}

func (a *DOVolumes) MyIP() (string, error) {
	addr, err := getMetadata(dropletInternalIPMetadataURL)
	if err != nil {
		return "", err
	}

	return addr, nil
}

func getMetadataRegion() (string, error) {
	return getMetadata(dropletRegionMetadataURL)
}

func getMetadataDropletName() (string, error) {
	return getMetadata(dropletNameMetadataURL)
}

func getMetadataDropletID() (string, error) {
	return getMetadata(dropletIDMetadataURL)
}

func getMetadataDropletTags() ([]string, error) {

	tagString, err := getMetadata(dropletIDMetadataTags)
	if err != nil {
		return nil, fmt.Errorf("error fetching droplet tags: %v", err)
	}

	dropletTags := strings.Split(tagString, "\n")
	return dropletTags, nil
}

func getMetadata(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("droplet metadata returned non-200 status code: %d", resp.StatusCode)
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(bodyBytes), nil
}

// NewCloud returns a Cloud, expecting the env var DIGITALOCEAN_ACCESS_TOKEN
// NewCloud will return an err if DIGITALOCEAN_ACCESS_TOKEN is not defined
func NewCloud(region string) (*DigiCloud, error) {
	accessToken := os.Getenv("DIGITALOCEAN_ACCESS_TOKEN")
	if accessToken == "" {
		return nil, errors.New("DIGITALOCEAN_ACCESS_TOKEN is required")
	}

	tokenSource := &TokenSource{
		AccessToken: accessToken,
	}

	oauthClient := oauth2.NewClient(oauth2.NoContext, tokenSource)
	client := godo.NewClient(oauthClient)

	return &DigiCloud{
		Client: client,
		Region: region,
	}, nil
}

func getAllVolumesByRegion(cloud *DigiCloud, region string, filterByRegion bool) ([]godo.Volume, error) {
	allVolumes := []godo.Volume{}

	opt := &godo.ListOptions{}

	listvolumeParams := &godo.ListVolumeParams{}

	if filterByRegion {
		listvolumeParams = &godo.ListVolumeParams{
			Region:      region,
			ListOptions: opt,
		}
	} else {
		listvolumeParams = &godo.ListVolumeParams{
			ListOptions: opt,
		}
	}

	for {
		volumes, resp, err := cloud.Client.Storage.ListVolumes(context.TODO(), listvolumeParams)

		if err != nil {
			return nil, err
		}

		allVolumes = append(allVolumes, volumes...)

		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}

		page, err := resp.Links.CurrentPage()
		if err != nil {
			return nil, err
		}

		opt.Page = page + 1

	}

	return allVolumes, nil

}

func getLocalDeviceName(vol *godo.Volume) string {
	return localDevicePrefix + vol.Name
}

func getVolumeByID(cloud *DigiCloud, id string) (*godo.Volume, error) {
	vol, _, err := cloud.Client.Storage.GetVolume(context.TODO(), id)
	return vol, err

}
