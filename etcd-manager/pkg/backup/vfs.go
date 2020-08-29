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

package backup

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"k8s.io/klog"
	"k8s.io/kops/util/pkg/vfs"
	"kope.io/etcd-manager/pkg/apis/etcd"
)

func NewVFSStore(p vfs.Path) (Store, error) {
	s := &vfsStore{
		spec:        p.Path(),
		backupsBase: p,
	}
	return s, nil
}

type vfsStore struct {
	spec        string
	backupsBase vfs.Path
}

var _ Store = &vfsStore{}

func (s *vfsStore) AddBackup(srcFile string, sequence string, info *etcd.BackupInfo) (string, error) {
	now := time.Now()

	if info.Timestamp == 0 {
		info.Timestamp = now.Unix()
	}

	name := now.UTC().Format(time.RFC3339) + "-" + sequence

	// Copy the backup file
	if srcFile != "" {
		f, err := os.Open(srcFile)
		if err != nil {
			return "", fmt.Errorf("error opening %q: %v", srcFile, err)
		}
		defer f.Close()

		destPath := s.backupsBase.Join(name).Join(DataFilename)
		err = destPath.WriteFile(f, nil)
		if err != nil {
			return "", fmt.Errorf("error copying %q to %q: %v", srcFile, destPath, err)
		}
	}

	// Save the meta file
	{
		p := s.backupsBase.Join(name, MetaFilename)

		data, err := etcd.ToJson(info)
		if err != nil {
			return "", fmt.Errorf("error marshalling state: %v", err)
		}

		err = p.WriteFile(bytes.NewReader([]byte(data)), nil)
		if err != nil {
			return "", fmt.Errorf("error writing file %q: %v", p, err)
		}
	}

	return name, nil
}

func (s *vfsStore) ListBackups() ([]string, error) {
	files, err := s.backupsBase.ReadTree()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error reading %s: %v", s.backupsBase, err)
	}

	var backups []string
	for _, f := range files {
		if f.Base() != MetaFilename {
			continue
		}

		path := f.Path()
		tokens := strings.Split(path, "/")
		if len(tokens) < 2 {
			klog.Infof("skipping unexpectedly short path %q", path)
			continue
		} else {
			backups = append(backups, tokens[len(tokens)-2])
		}
	}

	sort.Strings(backups)

	klog.Infof("listed backups in %s: %v", s.backupsBase, backups)

	return backups, nil
}

func (s *vfsStore) RemoveBackup(backup string) error {
	p := s.backupsBase.Join(backup)

	files, err := p.ReadTree()
	if err != nil {
		return fmt.Errorf("error deleting - cannot read %s: %v", p, err)
	}

	for _, f := range files {
		err := f.RemoveAllVersions()
		if err != nil {
			return fmt.Errorf("error deleting backups in %q: %v", p, err)
		}
	}

	return nil
}

func (s *vfsStore) LoadInfo(name string) (*etcd.BackupInfo, error) {
	klog.Infof("Loading info for backup %q", name)

	p := s.backupsBase.Join(name, MetaFilename)

	data, err := p.ReadFile()
	if err != nil {
		return nil, fmt.Errorf("error reading file %q: %v", p, err)
	}

	spec := &etcd.BackupInfo{}
	if err = etcd.FromJson(string(data), spec); err != nil {
		return nil, fmt.Errorf("error parsing file %q: %v", p, err)
	}

	klog.Infof("read backup info for %q: %v", name, spec)

	return spec, nil
}

func (s *vfsStore) Spec() string {
	return s.spec
}

func (s *vfsStore) DownloadBackup(name string, destFile string) error {
	klog.Infof("Downloading backup %q -> %s", name, destFile)

	dir := path.Dir(destFile)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return fmt.Errorf("error creating directories %q: %v", dir, err)
	}

	f, err := ioutil.TempFile(dir, "tmp")
	if err != nil {
		return fmt.Errorf("error creating temp file in %q: %v", dir, err)
	}
	tempfile := f.Name()
	defer func() {
		if f != nil {
			_ = f.Close()
		}
		if tempfile != "" {
			_ = os.Remove(tempfile)
		}
	}()

	srcPath := s.backupsBase.Join(name).Join(DataFilename)
	_, err = srcPath.WriteTo(f)
	if err != nil {
		return err
	}

	err = f.Close()
	f = nil
	if err != nil {
		return err
	}

	err = os.Rename(tempfile, destFile)
	if err != nil {
		return fmt.Errorf("error during file write of %q: rename failed: %v", destFile, err)
	}
	tempfile = ""

	return nil
}
