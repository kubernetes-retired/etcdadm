package preflight

import (
	"fmt"
	"log"

	"github.com/platform9/etcdadm/apis"
	"github.com/platform9/etcdadm/service"
)

// Mandatory runs the mandatory pre-flight checks, returning an error if any
// check fails.
func Mandatory(cfg *apis.EtcdAdmConfig) error {
	log.Println("[pre-flight] Comparing current and last used etcd versions")
	dv, err := service.DiffVersion(cfg)
	if err != nil {
		return fmt.Errorf("unable to compare current and last used etcd versions: %v", err)
	}
	if dv != "" {
		return fmt.Errorf("the current and last used etcd versions are different: %q, need to run etcdadm reset", dv)
	}
	log.Println("[pre-flight] Comparing current and last used etcd configurations")
	dc, err := service.DiffEnvironmentFile(cfg)
	if err != nil {
		return fmt.Errorf("unable to compare current and last used configurations: %v", err)
	}
	if len(dc) != 0 {
		return fmt.Errorf("current and last used configuration are different: %q, need to run etcdadm reset", dc)
	}
	log.Println("[pre-flight] The current and last configurations match.")
	return nil
}
