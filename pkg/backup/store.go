package backup

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
)

type Store interface {
	Spec() string

	CreateBackupTempDir(name string) (string, error)
	AddBackup(name string, srcdir string) error
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
		backupsBase: filepath.Join(base, "backups"),
	}

	if err := os.MkdirAll(s.backupsBase, 0700); err != nil {
		return nil, fmt.Errorf("error creating directories %q: %v", s.backupsBase, err)
	}

	return s, nil
}

type filesystemStore struct {
	spec        string
	backupsBase string
}

var _ Store = &filesystemStore{}

func (s *filesystemStore) CreateBackupTempDir(name string) (string, error) {
	p := filepath.Join(s.backupsBase, name+".tmp")
	err := os.Mkdir(p, 0700)
	if err != nil {
		return "", fmt.Errorf("unable to create backup temp directory %s: %v", p, err)
	}
	return p, nil
}

func (s *filesystemStore) AddBackup(name string, srcdir string) error {
	p := filepath.Join(s.backupsBase, name)
	err := os.Rename(srcdir, p)
	if err != nil {
		return fmt.Errorf("error renaming %q to %q: %v", srcdir, p, err)
	}
	return nil
}

func (s *filesystemStore) Spec() string {
	return s.spec
}
