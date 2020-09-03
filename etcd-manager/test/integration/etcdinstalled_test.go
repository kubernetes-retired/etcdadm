/*
Copyright 2020 The Kubernetes Authors.

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

package integration

import (
	"testing"

	"kope.io/etcd-manager/pkg/etcd"
	"kope.io/etcd-manager/pkg/etcdversions"
)

func TestEtcdInstalled(t *testing.T) {
	for _, etcdVersion := range etcdversions.AllEtcdVersions {
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
