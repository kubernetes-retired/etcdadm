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

// EnableAndStartService enables and starts the etcd service
func EnableAndStartService() error {
	if err := serviceEnable(unitFile); err != nil {
		return err
	}
	return serviceStart(unitFile)
}
