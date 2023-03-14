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

package main

import (
	"fmt"
	"net"
	"reflect"
	"testing"
)

func getTestData(networkCIDR string, volumeProviderID string) *EtcdManagerOptions {
	var o EtcdManagerOptions
	o.InitDefaults()

	o.ClusterName = "test"
	o.BackupStorePath = "s3://test"

	o.NetworkCIDR = networkCIDR
	o.VolumeProviderID = volumeProviderID

	return &o
}

func assertTestResults(t *testing.T, err error, expected interface{}, actual interface{}) {
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if !reflect.DeepEqual(expected, actual) {
		t.Fatalf("expected '%+v', but got '%+v'", expected, actual)
	}
}

func TestRunEtcdManagerReturnsNetworkCIDROnlySupportedWithOpenstack(t *testing.T) {
	expectedErr := fmt.Errorf("network-cidr is only supported with provider 'openstack'")
	o := getTestData("192.168.0.0/16", "aws")

	actualErr := RunEtcdManager(o)

	assertTestResults(t, nil, expectedErr, actualErr)
}

func TestRunEtcdManagerReturnsErrorOnInvalidCIDR(t *testing.T) {
	invalidCIDR := "192.168.0.0/123"
	expectedErr := fmt.Errorf("parsing network-cidr: %w", &net.ParseError{Type: "CIDR address", Text: invalidCIDR})
	o := getTestData(invalidCIDR, "openstack")

	actualErr := RunEtcdManager(o)

	assertTestResults(t, nil, expectedErr, actualErr)
}
