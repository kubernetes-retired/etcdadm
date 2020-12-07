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
	"context"
	"net"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2020-06-01/network"
)

type mockClient struct {
	dDisks []*dataDisk
	vms    map[string]compute.VirtualMachineScaleSetVM
	disks  []compute.Disk
	ifaces []network.Interface
}

var _ clientInterface = &mockClient{}

func newMockClient() *mockClient {
	return &mockClient{
		vms: map[string]compute.VirtualMachineScaleSetVM{},
	}
}

func (c *mockClient) name() string {
	return "master.masters.cluster.k8s.local_0"
}

func (c *mockClient) vmScaleSetInstanceID() string {
	return "0"
}

func (c *mockClient) dataDisks() []*dataDisk {
	return c.dDisks
}

func (c *mockClient) refreshMetadata() error {
	return nil
}

func (c *mockClient) localIP() net.IP {
	return net.ParseIP("10.0.0.1")
}

func (c *mockClient) listVMScaleSetVMs(ctx context.Context) ([]compute.VirtualMachineScaleSetVM, error) {
	var l []compute.VirtualMachineScaleSetVM
	for _, v := range c.vms {
		l = append(l, v)
	}
	return l, nil
}

func (c *mockClient) getVMScaleSetVM(ctx context.Context, instanceID string) (*compute.VirtualMachineScaleSetVM, error) {
	vm := c.vms[instanceID]
	return &vm, nil
}
func (c *mockClient) updateVMScaleSetVM(ctx context.Context, instanceID string, parameters compute.VirtualMachineScaleSetVM) error {
	c.vms[instanceID] = parameters
	return nil
}

func (c *mockClient) listDisks(ctx context.Context) ([]compute.Disk, error) {
	return c.disks, nil
}

func (c *mockClient) listVMSSNetworkInterfaces(ctx context.Context) ([]network.Interface, error) {
	return c.ifaces, nil
}
