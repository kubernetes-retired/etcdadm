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

package locking

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"k8s.io/klog"
)

type FSContentLock struct {
	p string
}

var _ Lock = &FSContentLock{}

type FSContentLockGuard struct {
	p    string
	file *os.File
	data []byte
	id   string
}

var _ LockGuard = &FSContentLockGuard{}

func NewFSContentLock(p string) (*FSContentLock, error) {
	return &FSContentLock{
		p: p,
	}, nil
}

func (l *FSContentLock) Acquire(ctx context.Context, id string) (LockGuard, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	data := &LockInfo{
		Holder:    id,
		Timestamp: time.Now().Unix(),
	}
	b, err := data.ToJSON()
	if err != nil {
		return nil, fmt.Errorf("error serializing lock info: %v", err)
	}

	hash := sha256.Sum256(b)
	lockBytes := []byte(hex.EncodeToString(hash[:]) + "\n" + string(b))

	f, err := os.OpenFile(l.p, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return nil, fmt.Errorf("error creating lock file %q: %v", l.p, err)
	}

	fileBytes, err := ioutil.ReadAll(f)
	if err != nil {
		if errC := f.Close(); errC != nil {
			klog.Fatalf("error closing lock file %q: %v", l.p, errC)
		}
		return nil, fmt.Errorf("error reading lock file: %v", err)
	}
	if len(fileBytes) != 0 {
		var existing *LockInfo
		firstLF := bytes.IndexByte(fileBytes, '\n')
		if firstLF != -1 {
			computedHash := sha256.Sum256(fileBytes[firstLF+1:])
			if string(fileBytes[0:firstLF]) == hex.EncodeToString(computedHash[:]) {
				existing = &LockInfo{}
				if errU := json.Unmarshal(fileBytes[firstLF+1:], existing); errU != nil {
					klog.Warningf("error parsing json %q - will treat as lock not held: %v", string(fileBytes), errU)
				} else {
					if errC := f.Close(); errC != nil {
						klog.Fatalf("error closing lock file %q: %v", l.p, errC)
					}
					klog.Warningf("lock is already held %v", existing)
					return nil, nil
				}
			} else {
				klog.Warningf("hash in file %q did not match: %q", l.p, string(fileBytes))
			}
		} else {
			klog.Warningf("file %q was corrupt: %q", l.p, string(fileBytes))
		}

		if errT := f.Truncate(0); errT != nil {
			if errC := f.Close(); errC != nil {
				klog.Fatalf("error closing lock file %q: %v", l.p, errC)
			}
			return nil, fmt.Errorf("failed to truncate file after failed read %q: %v", l.p, errT)
		}
	}

	_, err = f.WriteAt(lockBytes, 0)
	if err != nil {
		klog.Warningf("error writing file %q: %v", l.p, err)
	}

	if err == nil {
		err = f.Sync()
		if err != nil {
			klog.Warningf("failed to sync file: %v", err)
		}
	}

	if err != nil {
		klog.Warningf("attempting to truncate file after failed write %q: %v", l.p, err)
		if errT := f.Truncate(0); errT != nil {
			klog.Warningf("failed to truncate file after failed write %q: %v", l.p, errT)
		}
	}

	if errC := f.Close(); errC != nil {
		klog.Fatalf("error closing lock file %q: %v", l.p, errC)
	}

	if err != nil {
		return nil, fmt.Errorf("error creating lock file %q: %v", l.p, err)
	}

	klog.Infof("Acquired FSLock on %s for %s", l.p, id)

	return &FSContentLockGuard{
		p:    l.p,
		id:   id,
		file: f,
		data: lockBytes,
	}, nil
}

func (l *FSContentLockGuard) Release() error {
	f, errO := os.OpenFile(l.p, os.O_RDWR, 0755)
	if errO != nil {
		return fmt.Errorf("error opening file %q: %v", l.p, errO)
	}

	fileBytes, errR := ioutil.ReadAll(f)
	if errR != nil {
		if errC := f.Close(); errC != nil {
			klog.Fatalf("error closing lock file %q: %v", l.p, errC)
		}
		return fmt.Errorf("error reading lock file: %v", errR)
	}

	if !bytes.Equal(fileBytes, l.data) {
		if errC := f.Close(); errC != nil {
			klog.Fatalf("error closing lock file %q: %v", l.p, errC)
		}
		return fmt.Errorf("lock file %q changed", l.p)
	}

	if errT := f.Truncate(0); errT != nil {
		klog.Warningf("failed to truncate file as part of lock release %q: %v", l.p, errT)

		if errC := f.Close(); errC != nil {
			klog.Fatalf("error closing lock file %q: %v", l.p, errC)
		}

		return fmt.Errorf("failed to truncate file after failed write %q: %v", l.p, errT)
	}

	if errC := f.Close(); errC != nil {
		return fmt.Errorf("error closing file as part of unlock %q: %v", l.p, errC)
	}

	klog.Infof("Released FSLock on %q for %s", l.p, l.id)

	return nil
}
