package integration

import (
	"testing"

	"kope.io/etcd-manager/pkg/etcd"
)

func TestEtcdInstalled(t *testing.T) {
	versions := []string{"2.2.1", "3.2.12"}
	for _, version := range versions {
		bindir, err := etcd.BindirForEtcdVersion(version)
		if err != nil {
			t.Errorf("etcd %q not installed in /opt: %v", version, err)
		}
		if bindir == "" {
			t.Errorf("etcd %q did not return bindir", version)
		}
	}
}
