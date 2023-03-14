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

package openstack

import (
	"fmt"
	"net"
	"testing"
)

type TestData struct {
	clusterName string
	ips         []string
	addrs       map[string]interface{}
}

func getTestData() *TestData {
	ips := []string{
		"10.135.1.1",
		"2001:db8::1:0",
		"192.168.1.1",
		"192.168.2.1",
	}

	addrs := map[string]interface{}{
		"test_network_1": []interface{}{
			map[string]interface{}{
				"OS-EXT-IPS-MAC:mac_addr": "00:00:00:00:00:01",
				"OS-EXT-IPS:type":         "fixed",
				"addr":                    ips[0],
				"version":                 "4",
			},
			map[string]interface{}{
				"OS-EXT-IPS-MAC:mac_addr": "00:00:00:00:00:02",
				"OS-EXT-IPS:type":         "fixed",
				"addr":                    ips[1],
				"version":                 "6",
			},
		},
		"test_network_2": []interface{}{
			map[string]interface{}{
				"OS-EXT-IPS-MAC:mac_addr": "00:00:00:00:00:03",
				"OS-EXT-IPS:type":         "fixed",
				"addr":                    ips[2],
				"version":                 "4",
			},
			map[string]interface{}{
				"OS-EXT-IPS-MAC:mac_addr": "00:00:00:00:00:04",
				"OS-EXT-IPS:type":         "fixed",
				"addr":                    ips[3],
				"version":                 "4",
			},
		},
	}

	return &TestData{
		clusterName: "test",
		ips:         ips,
		addrs:       addrs,
	}
}

func TestReturnErrorOnEmptyAddresses(t *testing.T) {
	td := getTestData()
	td.addrs = map[string]interface{}{}

	expectedErr := fmt.Errorf("failed to find Fixed IP address for server %s", td.clusterName)

	_, actualErr := GetServerFixedIP(td.addrs, td.clusterName, nil)

	assertTestResults(t, nil, fmt.Sprintf("%+v", expectedErr), fmt.Sprintf("%+v", actualErr))
}

func TestReturnFirstFixedIP(t *testing.T) {
	td := getTestData()
	expectedIP := td.ips[0]

	actualIP, err := GetServerFixedIP(td.addrs, td.clusterName, nil)

	assertTestResults(t, err, expectedIP, actualIP)
}

func TestReturnErrorOnNonMatchingCIDR(t *testing.T) {
	td := getTestData()

	_, cidr, _ := net.ParseCIDR("172.16.0.0/16")

	expectedErr := fmt.Errorf("failed to find Fixed IP address for server %s", td.clusterName)

	_, actualErr := GetServerFixedIP(td.addrs, td.clusterName, cidr)

	assertTestResults(t, nil, expectedErr, actualErr)
}

func TestReturnFirstIPv4MatchingCIDR(t *testing.T) {
	td := getTestData()

	_, cidr, _ := net.ParseCIDR("192.168.2.0/24")

	expectedIP := td.ips[3]

	actualIP, err := GetServerFixedIP(td.addrs, td.clusterName, cidr)

	assertTestResults(t, err, expectedIP, actualIP)
}

func TestReturnFirstIPv6MatchingCIDR(t *testing.T) {
	td := getTestData()

	_, cidr, _ := net.ParseCIDR("2001:db8::/64")

	expectedIP := td.ips[1]

	actualIP, err := GetServerFixedIP(td.addrs, td.clusterName, cidr)

	assertTestResults(t, err, expectedIP, actualIP)
}
