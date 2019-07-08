/*
Copyright 2018 The Kubernetes Authors.

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

package preflight

import (
	"fmt"

	log "sigs.k8s.io/etcdadm/pkg/logrus"

	"sigs.k8s.io/etcdadm/apis"
	"sigs.k8s.io/etcdadm/service"
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
