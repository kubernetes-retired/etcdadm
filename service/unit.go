/*
Copyright 2018 The Kubernetes Authors.

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

package service

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"sigs.k8s.io/etcdadm/apis"
	"sigs.k8s.io/etcdadm/constants"
)

// WriteEnvironmentFile writes the environment file used by the etcd service unit
func WriteEnvironmentFile(cfg *apis.EtcdAdmConfig) error {
	t := template.Must(template.New("environment").Parse(constants.EnvFileTemplate))

	environmentFileDir := filepath.Dir(cfg.EnvironmentFile)
	if err := os.MkdirAll(environmentFileDir, 0755); err != nil {
		return fmt.Errorf("unable to create environment file directory %q: %s", environmentFileDir, err)
	}

	f, err := os.OpenFile(cfg.EnvironmentFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to open the etcd environment file %s: %s", cfg.EnvironmentFile, err)
	}
	defer f.Close()

	if err := t.Execute(f, cfg); err != nil {
		return fmt.Errorf("unable to apply the etcd environment: %s", err)
	}
	return nil
}

// WriteUnitFile writes etcd service unit file
func WriteUnitFile(cfg *apis.EtcdAdmConfig) error {
	t := template.Must(template.New("unit").Parse(constants.UnitFileTemplate))

	unitFileDir := filepath.Dir(cfg.UnitFile)
	if err := os.MkdirAll(unitFileDir, 0755); err != nil {
		return fmt.Errorf("unable to create unit file directory %q: %s", unitFileDir, err)
	}

	f, err := os.OpenFile(cfg.UnitFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("unable to open the etcd service unit file %s: %s", cfg.UnitFile, err)
	}
	defer f.Close()

	if err := t.Execute(f, cfg); err != nil {
		return fmt.Errorf("unable to apply etcd environment: %s", err)
	}
	return nil
}
