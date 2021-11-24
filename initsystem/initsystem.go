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

package initsystem

import (
	"fmt"
	"os/exec"
	"time"

	"sigs.k8s.io/etcdadm/apis"
	"sigs.k8s.io/etcdadm/initsystem/kubelet"
)

// InitSystem is the interface that describe behaviors of an init system
type InitSystem interface {
	Install() error
	Configure() error
	IsActive() (bool, error)
	EnableAndStartService() error
	DisableAndStopService() error
	StartupTimeout() time.Duration
}

// GetInitSystem returns an InitSystem for the current system, or error
// if we cannot detect a supported init system.
func GetInitSystem(config *apis.EtcdAdmConfig) (InitSystem, error) {
	switch config.InitSystem {
	case apis.Systemd:
		return systemd(config)
	case apis.Kubelet:
		return kubelet.New(config), nil
	default:
		return nil, fmt.Errorf("invalid init system %s", config.InitSystem)
	}
}

func systemd(config *apis.EtcdAdmConfig) (InitSystem, error) {
	_, err := exec.LookPath("systemctl")
	if err == nil {
		return &SystemdInitSystem{etcdAdmConfig: config}, nil
	}

	return nil, fmt.Errorf("systemd not detected; ensure that `systemctl` is in the PATH")
}
