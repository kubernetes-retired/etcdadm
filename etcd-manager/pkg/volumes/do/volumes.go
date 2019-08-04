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
	"errors"
	"fmt"
	"golang.org/x/oauth2"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	// "path/filepath"
	// "strings"
	// "sync"
	// "time"

	"github.com/digitalocean/godo"
	//"k8s.io/kops/pkg/resources/digitalocean"
	// "github.com/golang/glog"

	"kope.io/etcd-manager/pkg/volumes"
)

// DigiCloud exposes all the interfaces required to operate on DigitalOcean resources
type DigiCloud struct {
	Client *godo.Client
	Region string
	tags   map[string]string
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
	}, nil
}

func (a *DOVolumes) FindVolumes() ([]*volumes.Volume, error) {
	return nil, nil
}

// FindMountedVolume implements Volumes::FindMountedVolume
func (a *DOVolumes) FindMountedVolume(volume *volumes.Volume) (string, error) {
	return "", nil
}

// AttachVolume attaches the specified volume to this instance, returning the mountpoint & nil if successful
func (a *DOVolumes) AttachVolume(volume *volumes.Volume) error {
	return nil
}

func (a *DOVolumes) MyIP() (string, error) {
	return "", nil
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
