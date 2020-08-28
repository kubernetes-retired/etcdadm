package privateapi

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"k8s.io/klog"
	"kope.io/etcd-manager/pkg/ioutils"
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
