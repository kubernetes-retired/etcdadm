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
	"io"

	gcfg "gopkg.in/gcfg.v1"
)

type BlockStorageOpts struct {
	IgnoreVolumeAZ bool `gcfg:"ignore-volume-az"`
}

type Config struct {
	Global struct {
		AuthURL          string `gcfg:"auth-url"`
		Username         string
		UserID           string `gcfg:"user-id"`
		Password         string
		TenantID         string `gcfg:"tenant-id"`
		TenantName       string `gcfg:"tenant-name"`
		TrustID          string `gcfg:"trust-id"`
		DomainID         string `gcfg:"domain-id"`
		DomainName       string `gcfg:"domain-name"`
		TenantDomainID   string `gcfg:"tenant-domain-id"`
		TenantDomainName string `gcfg:"tenant-domain-name"`
		Region           string
		CAFile           string `gcfg:"ca-file"`
		Cloud            string `gcfg:"cloud,omitempty"`
	}
	BlockStorage BlockStorageOpts
}

func ReadConfig(config io.Reader) (Config, error) {
	cfg := Config{}
	err := gcfg.FatalOnly(gcfg.ReadInto(&cfg, config))
	return cfg, err
}
