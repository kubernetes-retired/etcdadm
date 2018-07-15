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

	f, err := os.OpenFile(cfg.EtcdctlEnvFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("unable to open the etcd environment file %s: %s", cfg.EtcdctlEnvFile, err)
	}
	defer f.Close()

	if err := t.Execute(f, cfg); err != nil {
		return fmt.Errorf("unable to apply the etcd environment: %s", err)
	}
	return nil
}
