package backup

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"sort"

	"io"

	"github.com/golang/glog"
	"kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/pkg/ioutils"
)

const MetaFilename = "_kopeio_etcd_manager.meta"

type Store interface {
	Spec() string

	CreateBackupTempDir(name string) (string, error)
	AddBackup(name string, srcdir string, info *etcd.BackupInfo) error

	// ListBackups returns all the available backups, in chronological order
	ListBackups() ([]string, error)

	// RemoveBackup deletes a backup (as returned by ListBackups)
	RemoveBackup(backup string) error

	// LoadInfo loads the backup information that should have been saved alongside a backup
	LoadInfo(backup string) (*etcd.BackupInfo, error)

	// DownloadBackup downloads the backup to the specific location
	DownloadBackup(name string, destdir string) error
}

func NewStore(storage string) (Store, error) {
	u, err := url.Parse(storage)
	if err != nil {
		return nil, fmt.Errorf("error parsing storage url %q", storage)
	}

	switch u.Scheme {
	case "file":
		return NewFilesystemStore(u)

	default:
		return nil, fmt.Errorf("unknown storage scheme %q", storage)
	}
}

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

func (s *filesystemStore) CreateBackupTempDir(name string) (string, error) {
	p := filepath.Join(s.tempBase, name)
	err := os.Mkdir(p, 0700)
	if err != nil {
		return "", fmt.Errorf("unable to create backup temp directory %s: %v", p, err)
	}
	return p, nil
}

func (s *filesystemStore) AddBackup(name string, srcdir string, info *etcd.BackupInfo) error {
	// Save the meta file
	{
		p := filepath.Join(srcdir, MetaFilename)

		data, err := etcd.ToJson(info)
		if err != nil {
			return fmt.Errorf("error marshalling state: %v", err)
		}

		err = ioutils.CreateFile(p, []byte(data), 0700)
		if err != nil {
			return fmt.Errorf("error writing file %q: %v", p, err)
		}
	}

	// Move the backup dir in place
	{
		p := filepath.Join(s.backupsBase, name)
		err := os.Rename(srcdir, p)
		if err != nil {
			return fmt.Errorf("error renaming %q to %q: %v", srcdir, p, err)
		}
	}

	return nil
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

func (s *filesystemStore) Spec() string {
	return s.spec
}

func (s *filesystemStore) DownloadBackup(name string, destdir string) error {
	p := filepath.Join(s.backupsBase, name)
	return copyTree(p, destdir)
}

func copyTree(srcdir string, destdir string) error {
	if err := os.MkdirAll(destdir, 0755); err != nil {
		return fmt.Errorf("error creating directory %s: %v", destdir, err)
	}

	srcfiles, err := ioutil.ReadDir(srcdir)
	if err != nil {
		return fmt.Errorf("error reading directory %s: %v", srcdir, err)
	}

	for _, srcfile := range srcfiles {
		if srcfile.IsDir() {
			if err := copyTree(filepath.Join(srcdir, srcfile.Name()), filepath.Join(destdir, srcfile.Name())); err != nil {
				return err
			}
		} else {
			if err := copyFile(filepath.Join(srcdir, srcfile.Name()), filepath.Join(destdir, srcfile.Name())); err != nil {
				return err
			}
		}
	}

	return nil
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
