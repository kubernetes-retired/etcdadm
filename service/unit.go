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

	if err := os.MkdirAll(constants.EnvironmentFileDir, 0755); err != nil {
		return fmt.Errorf("unable to create environment file directory %q: %s", constants.EnvironmentFileDir, err)
	}

	p := filepath.Join(constants.EnvironmentFileDir, constants.EnvironmentFile)
	f, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("unable to open the etcd environment file %s: %s", constants.EnvironmentFile, err)
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

	if err := os.MkdirAll(constants.UnitFileDir, 0755); err != nil {
		return fmt.Errorf("unable to create unit file directory %q: %s", constants.UnitFileDir, err)
	}

	p := filepath.Join(constants.UnitFileDir, constants.UnitFile)
	f, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return fmt.Errorf("unable to open the etcd service unit file %s: %s", constants.UnitFile, err)
	}
	defer f.Close()

	if err := t.Execute(f, cfg); err != nil {
		return fmt.Errorf("unable to apply etcd environment: %s", err)
	}
	return nil
}
