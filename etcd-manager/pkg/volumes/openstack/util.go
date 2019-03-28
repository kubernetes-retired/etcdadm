/*
Copyright 2019 The Kubernetes Authors.

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
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
)

const (
	openstackExternalIPType = "OS-EXT-IPS:type"
	openstackAddressFixed   = "fixed"
	openstackAddress        = "addr"
)

func GetServerFixedIP(server *servers.Server) (poolAddress string, err error) {
	for _, address := range server.Addresses {
		if addresses, ok := address.([]interface{}); ok {
			for _, addr := range addresses {
				addrMap := addr.(map[string]interface{})
				if addrType, ok := addrMap[openstackExternalIPType]; ok && addrType == openstackAddressFixed {
					if fixedIP, ok := addrMap[openstackAddress]; ok {
						if fixedIPStr, ok := fixedIP.(string); ok {
							return fixedIPStr, nil
						}
					}
				}
			}
		}
	}
	return "", fmt.Errorf("Failed to find Fixed IP address for server %s", server.Name)
}
