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
)

// SystemdInitSystem defines systemd init system
type SystemdInitSystem struct{}

func (s SystemdInitSystem) reloadSystemd() error {
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %v", err)
	}
	return nil
}

// Start a service
func (s SystemdInitSystem) start(service string) error {
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
func (s SystemdInitSystem) stop(service string) error {
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
func (s SystemdInitSystem) enable(service string) error {
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
func (s SystemdInitSystem) disable(service string) error {
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
func (s SystemdInitSystem) EnableAndStartService(service string) error {
	if err := s.enable(service); err != nil {
		return err
	}
	return s.start(service)
}

// DisableAndStopService disables and stops the etcd service
func (s SystemdInitSystem) DisableAndStopService(service string) error {
	enabled, err := s.isEnabled(service)
	if err != nil {
		return fmt.Errorf("error checking if etcd service is enabled: %w", err)
	}
	if enabled {
		if err := s.disable(service); err != nil {
			return err
		}
	}

	active, err := s.IsActive(service)
	if err != nil {
		return fmt.Errorf("error checking if etcd service is active: %w", err)
	}

	if active {
		if err := s.stop(service); err != nil {
			return err
		}
	}

	return nil
}

// IsActive checks if the systemd unit is active
func (s SystemdInitSystem) IsActive(service string) (bool, error) {
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
func (s SystemdInitSystem) isEnabled(service string) (bool, error) {
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
