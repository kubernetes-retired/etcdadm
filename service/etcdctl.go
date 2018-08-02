package service

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"github.com/platform9/etcdadm/apis"
	"github.com/platform9/etcdadm/constants"
)

// WriteEtcdctlEnvFile writes the environment file that can be used with the etcdctl client
func WriteEtcdctlEnvFile(cfg *apis.EtcdAdmConfig) error {
	t := template.Must(template.New("etcdctl-env").Parse(constants.EtcdctlEnvFileTemplate))

	environmentFileDir := filepath.Dir(cfg.EtcdctlEnvFile)
	if err := os.MkdirAll(environmentFileDir, 0755); err != nil {
		return fmt.Errorf("unable to create environment file directory %q: %s", environmentFileDir, err)
	}

	f, err := os.OpenFile(cfg.EtcdctlEnvFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to open the etcd environment file %s: %s", cfg.EtcdctlEnvFile, err)
	}
	defer f.Close()

	if err := t.Execute(f, cfg); err != nil {
		return fmt.Errorf("unable to apply the etcd environment: %s", err)
	}
	return nil
}

// WriteEtcdctlShellWrapper writes a shell script that loads the environment file before running etcdctl.
func WriteEtcdctlShellWrapper(cfg *apis.EtcdAdmConfig) error {
	t := template.Must(template.New("etcdctl-shell-wrapper").Parse(constants.EtcdctlShellWrapperTemplate))

	f, err := os.OpenFile(cfg.EtcdctlShellWrapper, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("unable to create the etcd shell wrapper %q: %v", cfg.EtcdctlShellWrapper, err)
	}
	defer f.Close()

	if err := t.Execute(f, cfg); err != nil {
		return fmt.Errorf("unable to apply the etcd environment: %v", err)
	}
	return nil
}
