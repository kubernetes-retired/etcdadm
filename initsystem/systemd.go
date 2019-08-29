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
func (s SystemdInitSystem) Start(service string) error {
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
func (s SystemdInitSystem) Stop(service string) error {
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
func (s SystemdInitSystem) Enable(service string) error {
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
func (s SystemdInitSystem) Disable(service string) error {
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
	if err := s.Enable(service); err != nil {
		return err
	}
	return s.Start(service)
}

// DisableAndStopService disables and stops the etcd service
func (s SystemdInitSystem) DisableAndStopService(service string) error {
	if err := s.Disable(service); err != nil {
		return err
	}
	return s.Stop(service)
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

// IsEnabled checks if the systemd unit is enabled
func (s SystemdInitSystem) IsEnabled(service string) (bool, error) {
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
