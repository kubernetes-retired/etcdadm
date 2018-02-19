package backup

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/golang/glog"
	"kope.io/etcd-manager/pkg/apis/etcd"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/pkg/ioutils"
)

func NewFilesystemStore(u *url.URL) (Store, error) {
	if u.Scheme != "file" {
		return nil, fmt.Errorf("unexpected scheme for file store %q", u.String())
	}

	base := u.Path

	s := &filesystemStore{
		spec:        u.String(),
		tempBase:    filepath.Join(base, "tmp"),
		backupsBase: filepath.Join(base, "backups"),
	}
	if err := os.MkdirAll(s.tempBase, 0700); err != nil {
		return nil, fmt.Errorf("error creating directories %q: %v", s.tempBase, err)
	}
	if err := os.MkdirAll(s.backupsBase, 0700); err != nil {
		return nil, fmt.Errorf("error creating directories %q: %v", s.backupsBase, err)
	}

	return s, nil
}

type filesystemStore struct {
	spec        string
	tempBase    string
	backupsBase string
}

var _ Store = &filesystemStore{}

func (s *filesystemStore) AddBackup(srcFile string, sequence string, info *etcd.BackupInfo) (string, error) {
	now := time.Now()

	if info.Timestamp == 0 {
		info.Timestamp = now.Unix()
	}

	name := now.UTC().Format(time.RFC3339) + "-" + sequence

	// Copy the backup file to the destination
	if srcFile != "" {
		destFile := filepath.Join(s.backupsBase, name, DataFilename)
		err := copyFile(srcFile, destFile)
		if err != nil {
			return "", fmt.Errorf("error copying %q to %q: %v", srcFile, destFile, err)
		}
	}

	// Save the meta file
	{
		p := filepath.Join(s.backupsBase, name, MetaFilename)

		data, err := etcd.ToJson(info)
		if err != nil {
			return "", fmt.Errorf("error marshalling state: %v", err)
		}

		err = ioutils.CreateFile(p, []byte(data), 0700)
		if err != nil {
			return "", fmt.Errorf("error writing file %q: %v", p, err)
		}
	}

	return name, nil
}

func (s *filesystemStore) ListBackups() ([]string, error) {
	files, err := ioutil.ReadDir(s.backupsBase)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %v", s.backupsBase, err)
	}

	var backups []string
	for _, f := range files {
		if !f.IsDir() {
			glog.Infof("skipping non-directory %s", filepath.Join(s.backupsBase, f.Name()))
			continue
		}

		backups = append(backups, f.Name())
	}

	sort.Strings(backups)

	return backups, nil
}

func (s *filesystemStore) RemoveBackup(backup string) error {
	p := filepath.Join(s.backupsBase, backup)
	stat, err := os.Stat(p)
	if err != nil {
		return fmt.Errorf("error getting stat for %q: %v", p, err)
	}

	if !stat.IsDir() {
		return fmt.Errorf("backup %q was not a directory", p)
	}

	if err := os.RemoveAll(p); err != nil {
		return fmt.Errorf("error deleting backups in %q: %v", p, err)
	}
	return nil
}

func (s *filesystemStore) LoadInfo(name string) (*etcd.BackupInfo, error) {
	p := filepath.Join(s.backupsBase, name, MetaFilename)

	data, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("error reading file %q: %v", p, err)
	}

	spec := &etcd.BackupInfo{}
	if err = etcd.FromJson(string(data), spec); err != nil {
		return nil, fmt.Errorf("error parsing file %q: %v", p, err)
	}

	return spec, nil
}

func (s *filesystemStore) SeedNewCluster(spec *protoetcd.ClusterSpec) error {
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

func (s *filesystemStore) Spec() string {
	return s.spec
}

func (s *filesystemStore) DownloadBackup(name string, destFile string) error {
	srcFile := filepath.Join(s.backupsBase, name, DataFilename)
	return copyFile(srcFile, destFile)
}

func copyFile(srcfile, destfile string) (err error) {
	in, err := os.Open(srcfile)
	if err != nil {
		return
	}
	defer func() {
		cerr := in.Close()
		if err == nil {
			err = cerr
		}
	}()
	out, err := os.Create(destfile)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	return
}
