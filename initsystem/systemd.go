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
	"errors"
	"fmt"
	"html/template"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"sigs.k8s.io/etcdadm/apis"
	"sigs.k8s.io/etcdadm/binary"
	"sigs.k8s.io/etcdadm/constants"
	"sigs.k8s.io/etcdadm/service"
)

const defaultEtcdStartupTimeout = 5 * time.Second

// SystemdInitSystem defines systemd init system
type SystemdInitSystem struct {
	etcdAdmConfig *apis.EtcdAdmConfig
}

func (s SystemdInitSystem) reloadSystemd() error {
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %v", err)
	}
	return nil
}

// Start a service
func (s SystemdInitSystem) start() error {
	service := s.unitName()
	// Before we try to start any service, make sure that systemd is ready
	if err := s.reloadSystemd(); err != nil {
		return err
	}
	args := []string{"start", service}
	if err := exec.Command("systemctl", args...).Run(); err != nil {
		return fmt.Errorf("failed to start service: %v", err)
	}
	return nil
}

// Stop a service
func (s SystemdInitSystem) stop() error {
	service := s.unitName()
	// Before we try to start any service, make sure that systemd is ready
	if err := s.reloadSystemd(); err != nil {
		return err
	}
	args := []string{"stop", service}
	if err := exec.Command("systemctl", args...).Run(); err != nil {
		return fmt.Errorf("failed to stop service: %v", err)
	}
	return nil
}

// Enable a service
func (s SystemdInitSystem) enable() error {
	service := s.unitName()
	// Before we try to enable any service, make sure that systemd is ready
	if err := s.reloadSystemd(); err != nil {
		return err
	}
	args := []string{"enable", service}
	if err := exec.Command("systemctl", args...).Run(); err != nil {
		return fmt.Errorf("failed to enable service: %v", err)
	}
	return nil
}

// Disable a service
func (s SystemdInitSystem) disable() error {
	service := s.unitName()
	// Before we try to disable any service, make sure that systemd is ready
	if err := s.reloadSystemd(); err != nil {
		return err
	}
	args := []string{"disable", service}
	if err := exec.Command("systemctl", args...).Run(); err != nil {
		return fmt.Errorf("failed to disable service: %v", err)
	}
	return nil
}

// EnableAndStartService enables and starts the etcd service
func (s SystemdInitSystem) EnableAndStartService() error {
	if err := s.enable(); err != nil {
		return err
	}
	return s.start()
}

// DisableAndStopService disables and stops the etcd service
func (s SystemdInitSystem) DisableAndStopService() error {
	enabled, err := s.isEnabled()
	if err != nil {
		return fmt.Errorf("error checking if etcd service is enabled: %w", err)
	}
	if enabled {
		if err := s.disable(); err != nil {
			return err
		}
	}

	active, err := s.IsActive()
	if err != nil {
		return fmt.Errorf("error checking if etcd service is active: %w", err)
	}

	if active {
		if err := s.stop(); err != nil {
			return err
		}
	}

	return nil
}

// IsActive checks if the systemd unit is active
func (s SystemdInitSystem) IsActive() (bool, error) {
	service := s.unitName()
	args := []string{"is-active", service}
	if err := exec.Command("systemctl", args...).Run(); err != nil {
		switch v := err.(type) {
		case *exec.Error:
			return false, fmt.Errorf("failed to run command %q: %s", v.Name, v.Err)
		case *exec.ExitError:
			return false, nil
		default:
			return false, err
		}
	}
	return true, nil
}

// isEnabled checks if the systemd unit is enabled
func (s SystemdInitSystem) isEnabled() (bool, error) {
	service := s.unitName()
	args := []string{"is-enabled", service}
	if err := exec.Command("systemctl", args...).Run(); err != nil {
		switch v := err.(type) {
		case *exec.Error:
			return false, fmt.Errorf("failed to run command %q: %s", v.Name, v.Err)
		case *exec.ExitError:
			return false, nil
		default:
			return false, err
		}
	}
	return true, nil
}

// Install downloads the necessary bianries (if not in cache already) in moves them to the right directory
func (s SystemdInitSystem) Install() error {
	inCache, err := binary.InstallFromCache(s.etcdAdmConfig.Version, s.etcdAdmConfig.InstallDir, s.etcdAdmConfig.CacheDir)
	if err != nil {
		return fmt.Errorf("artifact could not be installed from cache: %w", err)
	}
	if !inCache {
		log.Printf("[install] Artifact not found in cache. Trying to fetch from upstream: %s", s.etcdAdmConfig.ReleaseURL)
		if err = binary.Download(s.etcdAdmConfig.ReleaseURL, s.etcdAdmConfig.Version, s.etcdAdmConfig.CacheDir); err != nil {
			return fmt.Errorf("unable to fetch artifact from upstream: %w", err)
		}
		// Try installing binaries from cache now
		inCache, err := binary.InstallFromCache(s.etcdAdmConfig.Version, s.etcdAdmConfig.InstallDir, s.etcdAdmConfig.CacheDir)
		if err != nil {
			return fmt.Errorf("artifact could not be installed from cache: %w", err)
		}
		if !inCache {
			return errors.New("artifact not found in cache after download")
		}
	}
	installed, err := binary.IsInstalled(s.etcdAdmConfig.Version, s.etcdAdmConfig.InstallDir)
	if err != nil {
		return fmt.Errorf("failed checking if binary is installed: %w", err)
	}
	if !installed {
		return fmt.Errorf("binaries not found in install dir")
	}

	return nil
}

// Configure boostraps the necessary configuration files for the etcd service
func (s SystemdInitSystem) Configure() error {
	if err := service.WriteEnvironmentFile(s.etcdAdmConfig); err != nil {
		return fmt.Errorf("failed writing environment file for etcd service: %w", err)
	}

	if err := s.writeUnitFile(); err != nil {
		return err
	}

	return nil
}

// writeUnitFile writes etcd service unit file
func (s SystemdInitSystem) writeUnitFile() error {
	t := template.Must(template.New("unit").Parse(constants.UnitFileTemplate))

	unitFileDir := filepath.Dir(s.etcdAdmConfig.UnitFile)
	if err := os.MkdirAll(unitFileDir, 0755); err != nil {
		return fmt.Errorf("unable to create unit file directory %q: %s", unitFileDir, err)
	}

	f, err := os.OpenFile(s.etcdAdmConfig.UnitFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("unable to open the etcd service unit file %s: %s", s.etcdAdmConfig.UnitFile, err)
	}
	defer f.Close()

	if err := t.Execute(f, s.etcdAdmConfig); err != nil {
		return fmt.Errorf("unable to apply etcd environment: %s", err)
	}
	return nil
}

// StartupTimeout defines the max time that the system should wait for etcd to be up
func (s SystemdInitSystem) StartupTimeout() time.Duration {
	return defaultEtcdStartupTimeout
}

// unitName is the name of the unit (service) in system
func (s SystemdInitSystem) unitName() string {
	name := filepath.Base(s.etcdAdmConfig.UnitFile)
	return name
}
