package backup

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/golang/glog"
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
		srcPath := vfs.NewFSPath(srcFile)
		destPath := s.backupsBase.Join(name).Join(DataFilename)
		if err := vfs.CopyFile(srcPath, destPath, nil); err != nil {
			return "", fmt.Errorf("error copying %q to %q: %v", srcPath, destPath, err)
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
			glog.Infof("skipping unexpectedly short path %q", path)
			continue
		} else {
			backups = append(backups, tokens[len(tokens)-2])
		}
	}

	sort.Strings(backups)

	glog.Infof("listed backups in %s: %v", s.backupsBase, backups)

	return backups, nil
}

func (s *vfsStore) RemoveBackup(backup string) error {
	p := s.backupsBase.Join(backup)

	files, err := p.ReadTree()
	if err != nil {
		return fmt.Errorf("error deleting - cannot read %s: %v", p, err)
	}

	for _, f := range files {
		err := f.Remove()
		if err != nil {
			return fmt.Errorf("error deleting backups in %q: %v", p, err)
		}
	}

	return nil
}

func (s *vfsStore) LoadInfo(name string) (*etcd.BackupInfo, error) {
	glog.Infof("Loading info for backup %q", name)

	p := s.backupsBase.Join(name, MetaFilename)

	data, err := p.ReadFile()
	if err != nil {
		return nil, fmt.Errorf("error reading file %q: %v", p, err)
	}

	spec := &etcd.BackupInfo{}
	if err = etcd.FromJson(string(data), spec); err != nil {
		return nil, fmt.Errorf("error parsing file %q: %v", p, err)
	}

	glog.Infof("read backup info for %q: %v", name, spec)

	return spec, nil
}

func (s *vfsStore) Spec() string {
	return s.spec
}

func (s *vfsStore) DownloadBackup(name string, destFile string) error {
	glog.Infof("Downloading backup %q -> %s", name, destFile)

	srcPath := s.backupsBase.Join(name).Join(DataFilename)
	destPath := vfs.NewFSPath(destFile)
	return vfs.CopyFile(srcPath, destPath, nil)
}
