package backup

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/golang/glog"
	"k8s.io/kops/util/pkg/vfs"
	"kope.io/etcd-manager/pkg/apis/etcd"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
)

func NewVFSStore(p vfs.Path) (Store, error) {
	s := &vfsStore{
		spec:        p.Path(),
		backupsBase: p.Join("backups"),
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
		if err := vfsCopyFile(srcPath, destPath, nilACLOracle); err != nil {
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

		err = p.WriteFile([]byte(data), nil)
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

	glog.Infof("listed backups: %v", backups)

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

func (s *vfsStore) SeedNewCluster(spec *protoetcd.ClusterSpec) error {
	backups, err := s.ListBackups()
	if err != nil {
		return fmt.Errorf("error listing backups: %v", err)
	}
	if len(backups) != 0 {
		return fmt.Errorf("cannot seed new cluster - cluster backups already exists")
	}

	tmpdir, err := ioutil.TempDir("", "")
	if err != nil {
		return err
	}

	defer func() {
		err := os.RemoveAll(tmpdir)
		if err != nil {
			glog.Warningf("error deleting backup temp directory %q: %v", tmpdir, err)
		}
	}()

	info := &etcd.BackupInfo{
		ClusterSpec: spec,
	}
	sequence := "000000"
	name, err := s.AddBackup("", sequence, info)
	if err != nil {
		return err
	}
	glog.Infof("created seed backup with name %q", name)

	return nil
}

func (s *vfsStore) Spec() string {
	return s.spec
}

func (s *vfsStore) DownloadBackup(name string, destFile string) error {
	glog.Infof("Downloading backup %q -> %s", name, destFile)

	srcPath := s.backupsBase.Join(name).Join(DataFilename)
	destPath := vfs.NewFSPath(destFile)
	return vfsCopyFile(srcPath, destPath, nilACLOracle)
}

// TODO: Move to vfs
var nilACLOracle = func(vfs.Path) (vfs.ACL, error) { return nil, nil }

// vfsCopyFile copies the file from src to dest.
func vfsCopyFile(srcFile vfs.Path, destFile vfs.Path, aclOracle vfs.ACLOracle) error {
	srcData, err := srcFile.ReadFile()
	if err != nil {
		return fmt.Errorf("error reading source file %q: %v", srcFile, err)
	}

	acl, err := aclOracle(destFile)
	if err != nil {
		return err
	}

	// TODO: Don't buffer in memory
	glog.V(2).Infof("Copying data (size = %d) from %s to %s", len(srcData), srcFile, destFile)
	if err := destFile.WriteFile(srcData, acl); err != nil {
		return fmt.Errorf("error writing dest file %q: %v", destFile, err)
	}

	return nil
}
