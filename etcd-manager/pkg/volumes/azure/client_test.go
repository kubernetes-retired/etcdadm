/*
Copyright 2020 The Kubernetes Authors.

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

package azure

import (
	"net"
	"os"
	"reflect"
	"testing"
)

func TestUnmarshalMetadata(t *testing.T) {
	data, err := os.ReadFile("testdata/metadata.json")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	actual, err := unmarshalInstanceMetadata(data)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	expected := &instanceMetadata{
		Compute: &instanceComputeMetadata{
			Name:              "examplevmname",
			ResourceGroupName: "macikgo-test-may-23",
			VMScaleSetName:    "crpteste9vflji9",
			SubscriptionID:    "8d10da13-8125-4ba9-a717-bf7490507b3d",
			StorageProfile: &storageProfile{
				DataDisks: []*dataDisk{
					{
						Name: "exampledatadiskname",
						Lun:  "0",
						ManagedDisk: &managedDisk{
							ID: "/subscriptions/8d10da13-8125-4ba9-a717-bf7490507b3d/resourceGroups/macikgo-test-may-23/providers/Microsoft.Compute/disks/exampledatadiskname",
						},
					},
				},
			},
		},
		Network: &instanceNetworkMetadata{
			Interfaces: []*networkInterface{
				{
					IPv4: &ipv4Interface{
						IPAddresses: []*ipAddress{
							{
								PrivateIPAddress: "172.16.32.8",
								PublicIPAddress:  "52.136.124.5",
							},
						},
					},
				},
			},
		},
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("expected %+v, but got %+v", expected, actual)
	}
}

func TestGetInternalIP(t *testing.T) {
	data, err := os.ReadFile("testdata/metadata.json")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	metadata, err := unmarshalInstanceMetadata(data)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	c := client{
		metadata: metadata,
	}

	if a, e := c.localIP(), net.ParseIP("172.16.32.8"); !a.Equal(e) {
		t.Errorf("expected local IP %s, but got %s", e, a)
	}
}
