package service

import (
	"fmt"
	"os/exec"
)

func reloadSystemd() error {
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %v", err)
	}
	return nil
}

func resetFailed() error {
	if err := exec.Command("systemctl", "reset-failed").Run(); err != nil {
		return fmt.Errorf("failed to reset failed systemd units: %v", err)
	}
	return nil
}

func serviceStart(service string) error {
	// Before we try to start any service, make sure that systemd is ready
	if err := reloadSystemd(); err != nil {
		return err
	}
	args := []string{"start", service}
	if err := exec.Command("systemctl", args...).Run(); err != nil {
		return fmt.Errorf("failed to start service: %v", err)
	}
	return nil
}

func serviceStop(service string) error {
	// Before we try to start any service, make sure that systemd is ready
	if err := reloadSystemd(); err != nil {
		return err
	}
	args := []string{"stop", service}
	if err := exec.Command("systemctl", args...).Run(); err != nil {
		return fmt.Errorf("failed to stop service: %v", err)
	}
	return nil
}

func serviceEnable(service string) error {
	// Before we try to enable any service, make sure that systemd is ready
	if err := reloadSystemd(); err != nil {
		return err
	}
	args := []string{"enable", service}
	if err := exec.Command("systemctl", args...).Run(); err != nil {
		return fmt.Errorf("failed to enable service: %v", err)
	}
	return nil
}

func serviceDisable(service string) error {
	// Before we try to disable any service, make sure that systemd is ready
	if err := reloadSystemd(); err != nil {
		return err
	}
	args := []string{"disable", service}
	if err := exec.Command("systemctl", args...).Run(); err != nil {
		return fmt.Errorf("failed to disable service: %v", err)
	}
	return nil
}

// EnableAndStartService enables and starts the etcd service
func EnableAndStartService(service string) error {
	if err := serviceEnable(service); err != nil {
		return err
	}
	return serviceStart(service)
}

// DisableAndStopService disables and stops the etcd service
func DisableAndStopService(service string) error {
	if err := serviceDisable(service); err != nil {
		return err
	}
	if err := serviceStop(service); err != nil {
		return err
	}
	return resetFailed()
}
