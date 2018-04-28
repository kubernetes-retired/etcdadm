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
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
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

func (s *vfsStore) AddCommand(cmd *protoetcd.Command) error {
	sequence := "000000"
	now := time.Now()

	cmd.Timestamp = now.UnixNano()

	name := now.UTC().Format(time.RFC3339) + "-" + sequence

	// Save the command file
	{
		p := s.backupsBase.Join("commands", name, CommandFilename)

		data, err := etcd.ToJson(cmd)
		if err != nil {
			return fmt.Errorf("error marshalling command: %v", err)
		}

		glog.Infof("Adding command at %s: %v", p, cmd)

		err = p.WriteFile(bytes.NewReader([]byte(data)), nil)
		if err != nil {
			return fmt.Errorf("error writing file %q: %v", p.Path(), err)
		}
	}

	return nil
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

func (s *vfsStore) ListCommands() ([]*Command, error) {
	commandsBase := s.backupsBase.Join("commands")
	files, err := commandsBase.ReadTree()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error reading %s: %v", commandsBase.Path(), err)
	}

	var commands []*Command
	for _, f := range files {
		if f.Base() != CommandFilename {
			continue
		}

		data, err := f.ReadFile()
		if err != nil {
			return nil, fmt.Errorf("error reading %s: %v", f, err)
		}

		command := &Command{}
		if err = etcd.FromJson(string(data), &command.Data); err != nil {
			return nil, fmt.Errorf("error parsing command %q: %v", f, err)
		}
		command.p = f

		glog.Infof("read command for %q: %v", f, command.Data.String())

		commands = append(commands, command)
	}

	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Data.Timestamp < commands[j].Data.Timestamp
	})

	glog.Infof("listed commands in %s: %d commands", commandsBase.Path(), len(commands))

	return commands, nil
}

func (s *vfsStore) RemoveCommand(command *Command) error {
	p := command.p
	glog.Infof("deleting command %s", p)

	if err := p.Remove(); err != nil {
		return fmt.Errorf("error removing command %s: %v", p, err)
	}

	return nil
}
