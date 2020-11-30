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

package privateapi

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"k8s.io/klog/v2"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/ioutils"
)

// PersistentPeerId reads the id from the base directory, creating and saving it if it does not exists
func PersistentPeerId(basedir string) (PeerId, error) {
	idFile := filepath.Join(basedir, "myid")

	b, err := ioutil.ReadFile(idFile)
	if err != nil {
		if os.IsNotExist(err) {
			token := randomToken()
			klog.Infof("Self-assigned new identity: %q", token)
			b = []byte(token)

			if err := os.MkdirAll(basedir, 0755); err != nil {
				return "", fmt.Errorf("error creating directories %q: %v", basedir, err)
			}

			if err := ioutils.CreateFile(idFile, b, 0644); err != nil {
				return "", fmt.Errorf("error creating id file %q: %v", idFile, err)
			}
		} else {
			return "", fmt.Errorf("error reading id file %q: %v", idFile, err)
		}
	}

	uniqueID := PeerId(string(b))
	return uniqueID, nil
}
