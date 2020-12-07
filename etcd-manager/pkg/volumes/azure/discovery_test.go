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
	"reflect"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2020-06-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/privateapi/discovery"
)

func newTestInterface(vmID, ip string) network.Interface {
	return network.Interface{
		InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
			VirtualMachine: &network.SubResource{
				ID: to.StringPtr(vmID),
			},
			IPConfigurations: &[]network.InterfaceIPConfiguration{
				{
					InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
						PrivateIPAddress: to.StringPtr(ip),
					},
				},
			},
		},
	}
}

func TestPoll(t *testing.T) {
	client := newMockClient()
	a, err := newAzureVolumes("cluster", []string{}, "nameTag", client)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	client.disks = []compute.Disk{
		{
			Name: to.StringPtr("name0"),
			ID:   to.StringPtr("did0"),
			DiskProperties: &compute.DiskProperties{
				DiskState: compute.DiskState("state"),
			},
			ManagedBy: to.StringPtr("vm_0"),
		},
		{
			Name: to.StringPtr("name1"),
			ID:   to.StringPtr("did1"),
			DiskProperties: &compute.DiskProperties{
				DiskState: compute.DiskState("state"),
			},
			ManagedBy: to.StringPtr("vm_1"),
		},
		{
			// Unmanaged disk.
			Name: to.StringPtr("name2"),
			ID:   to.StringPtr("did2"),
			DiskProperties: &compute.DiskProperties{
				DiskState: compute.DiskState("state"),
			},
		},
	}
	client.vms = map[string]compute.VirtualMachineScaleSetVM{
		"0": {
			Name: to.StringPtr("vm_0"),
			ID:   to.StringPtr("vmid0"),
		},
		"1": {
			Name: to.StringPtr("vm_1"),
			ID:   to.StringPtr("vmid1"),
		},
		"2": {
			Name: to.StringPtr("vm_2"),
			ID:   to.StringPtr("vmid2"),
		},
	}
	client.ifaces = []network.Interface{
		newTestInterface("vmid0", "10.0.0.1"),
		newTestInterface("vmid1", "10.0.0.2"),
		newTestInterface("vmid2", "10.0.0.3"),
	}

	actual, err := a.Poll()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	expected := map[string]discovery.Node{
		"name0": {
			ID: "name0",
			Endpoints: []discovery.NodeEndpoint{
				{
					IP: "10.0.0.1",
				},
			},
		},
		"name1": {
			ID: "name1",
			Endpoints: []discovery.NodeEndpoint{
				{
					IP: "10.0.0.2",
				},
			},
		},
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("expected %+v, but got +%v", expected, actual)
	}
}
