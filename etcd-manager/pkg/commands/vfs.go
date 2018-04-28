package commands

import (
	"bytes"
	"fmt"
	"os"
	"sort"
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

type vfsCommand struct {
	p    vfs.Path
	data protoetcd.Command
}

var _ Command = &vfsCommand{}

func (c *vfsCommand) Data() protoetcd.Command {
	return c.data
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

func (s *vfsStore) ListCommands() ([]Command, error) {
	commandsBase := s.backupsBase.Join("commands")
	files, err := commandsBase.ReadTree()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error reading %s: %v", commandsBase.Path(), err)
	}

	var commands []Command
	for _, f := range files {
		if f.Base() != CommandFilename {
			continue
		}

		data, err := f.ReadFile()
		if err != nil {
			return nil, fmt.Errorf("error reading %s: %v", f, err)
		}

		command := &vfsCommand{}
		if err = etcd.FromJson(string(data), &command.data); err != nil {
			return nil, fmt.Errorf("error parsing command %q: %v", f, err)
		}
		command.p = f

		glog.Infof("read command for %q: %v", f, command.data.String())

		commands = append(commands, command)
	}

	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Data().Timestamp < commands[j].Data().Timestamp
	})

	glog.Infof("listed commands in %s: %d commands", commandsBase.Path(), len(commands))

	return commands, nil
}

func (s *vfsStore) RemoveCommand(command Command) error {
	p := command.(*vfsCommand).p
	glog.Infof("deleting command %s", p)

	if err := p.Remove(); err != nil {
		return fmt.Errorf("error removing command %s: %v", p, err)
	}

	return nil
}
