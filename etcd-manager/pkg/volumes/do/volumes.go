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
	"golang.org/x/oauth2"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

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
	ClusterName string
	DigiCloud   *DigiCloud
	region      string
	dropletName string
	dropletID   int
	nameTag     string
}

var _ volumes.Volumes = &DOVolumes{}

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

	cloud, err := NewCloud(region)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize digitalocean cloud: %s", err)
	}

	return &DOVolumes{
		DigiCloud:   cloud,
		ClusterName: clusterName,
		dropletID:   dropletIDInt,
		dropletName: dropletName,
		region:      region,
		nameTag:     nameTag,
	}, nil
}

func (a *DOVolumes) FindVolumes() ([]*volumes.Volume, error) {
	return a.findVolumes(true)
}

func (a *DOVolumes) findVolumes(filterByRegion bool) ([]*volumes.Volume, error) {
	doVolumes, err := getAllVolumesByRegion(a.DigiCloud, a.region, filterByRegion)
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes: %s", err)
	}

	var myvolumes []*volumes.Volume
	for _, doVolume := range doVolumes {

		glog.V(2).Infof("Iterating DO Volume with name=%s and ID=%s", doVolume.Name, doVolume.ID)

		// determine if this volume belongs to this cluster
		// check for string a.ClusterName but with strings "." replaced with "-"
		// example volume name will be like kops-1-etcd-events-dev5-techthreads-co-in
		if !strings.Contains(doVolume.Name, strings.Replace(a.ClusterName, ".", "-", -1)) {
			continue
		}

		// Todo - Utilize volume tags once the change is done on the KOPS side. This still works since we specify a unique
		// volume name when creating a volume via kOPS.
		var clusterKey string
		if strings.Contains(doVolume.Name, "etcd-main") {
			clusterKey = "main"
		} else if strings.Contains(doVolume.Name, "etcd-events") {
			clusterKey = "events"
		} else {
			clusterKey = ""
			glog.V(2).Infof("could not determine etcd cluster type for volume: %s", doVolume.Name)
		}

		if strings.Contains(a.nameTag, clusterKey) {
			glog.V(2).Infof("Found a matching nameTag=%s", a.nameTag)
			vol := &volumes.Volume{
				ProviderID: doVolume.ID,
				Info: volumes.VolumeInfo{
					Description: a.nameTag,
				},
				MountName: "master-" + doVolume.ID,
				EtcdName:  a.ClusterName + "-" + clusterKey,
			}

			if len(doVolume.DropletIDs) == 1 {
				vol.AttachedTo = strconv.Itoa(doVolume.DropletIDs[0])
				vol.LocalDevice = getLocalDeviceName(&doVolume)
			}

			glog.V(2).Infof("Matching DO Volume found with name=%s ID=%s etcdname=%s mountname=%s", doVolume.Name, doVolume.ID, vol.EtcdName, vol.MountName)

			myvolumes = append(myvolumes, vol)
		}

	}

	return myvolumes, nil
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

		if len(doVolume.DropletIDs) == 1 {
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
	for {
		if filterByRegion {
			volumes, resp, err := cloud.Client.Storage.ListVolumes(context.TODO(), &godo.ListVolumeParams{
				Region:      region,
				ListOptions: opt,
			})

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

		} else {
			volumes, resp, err := cloud.Client.Storage.ListVolumes(context.TODO(), &godo.ListVolumeParams{
				ListOptions: opt,
			})

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
