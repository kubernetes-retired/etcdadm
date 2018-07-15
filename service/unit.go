package service

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"github.com/platform9/etcdadm/apis"
	"github.com/platform9/etcdadm/constants"
)

// WriteEnvironmentFile writes the environment file used by the etcd service unit
func WriteEnvironmentFile(cfg *apis.EtcdAdmConfig) error {
	t := template.Must(template.New("environment").Parse(constants.EnvFileTemplate))

	environmentFileDir := filepath.Dir(cfg.EnvironmentFile)
	if err := os.MkdirAll(environmentFileDir, 0755); err != nil {
		return fmt.Errorf("unable to create environment file directory %q: %s", environmentFileDir, err)
	}

	f, err := os.OpenFile(cfg.EnvironmentFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
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
