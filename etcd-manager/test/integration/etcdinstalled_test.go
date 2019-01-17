package integration

import (
	"testing"

	"kope.io/etcd-manager/pkg/etcd"
)

func TestEtcdInstalled(t *testing.T) {
	for _, etcdVersion := range AllEtcdVersions {
		t.Run("etcdVersion="+etcdVersion, func(t *testing.T) {
			{
				bindir, err := etcd.BindirForEtcdVersion(etcdVersion, "etcd")
				if err != nil {
					t.Errorf("etcd %q not installed in /opt: %v", etcdVersion, err)
				}
				if bindir == "" {
					t.Errorf("etcd %q did not return bindir", etcdVersion)
				}
			}
			{
				bindir, err := etcd.BindirForEtcdVersion(etcdVersion, "etcdctl")
				if err != nil {
					t.Errorf("etcdctl %q not installed in /opt: %v", etcdVersion, err)
				}
				if bindir == "" {
					t.Errorf("etcdctl %q did not return bindir", etcdVersion)
				}
			}
		})
	}
}
