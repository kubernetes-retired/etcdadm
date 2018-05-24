package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/golang/glog"
	"k8s.io/kops/util/pkg/vfs"
	"kope.io/etcd-manager/pkg/apis/etcd"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
)

const EtcdClusterCreated = "etcd-cluster-created"
const EtcdClusterSpec = "etcd-cluster-spec"

func NewVFSStore(p vfs.Path) (Store, error) {
	s := &vfsStore{
		commandsBase: p.Join("control"),
	}
	return s, nil
}

type vfsStore struct {
	commandsBase vfs.Path
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
		data, err := etcd.ToJson(cmd)
		if err != nil {
			return fmt.Errorf("error marshalling command: %v", err)
		}

		p := s.commandsBase.Join(name, CommandFilename)
		glog.Infof("Adding command at %s: %v", p, cmd)
		if err := p.WriteFile(bytes.NewReader([]byte(data)), nil); err != nil {
			return fmt.Errorf("error writing file %q: %v", p.Path(), err)
		}
	}

	return nil
}

func (s *vfsStore) ListCommands() ([]Command, error) {
	files, err := s.commandsBase.ReadTree()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error reading %s: %v", s.commandsBase.Path(), err)
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

	glog.Infof("listed commands in %s: %d commands", s.commandsBase.Path(), len(commands))

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

func (s *vfsStore) GetExpectedClusterSpec() (*protoetcd.ClusterSpec, error) {
	p := s.commandsBase.Join(EtcdClusterSpec)
	data, err := p.ReadFile()
	if err != nil {
		if os.IsNotExist(err) {
			// TODO: On S3, loop to try to establish consistency?
			return nil, nil
		}
		return nil, fmt.Errorf("error reading cluster spec file %s: %v", p.Path(), err)
	}

	spec := &protoetcd.ClusterSpec{}
	if err = etcd.FromJson(string(data), spec); err != nil {
		return nil, fmt.Errorf("error parsing cluster spec %s: %v", p.Path(), err)
	}

	return spec, nil
}

func (s *vfsStore) SetExpectedClusterSpec(spec *protoetcd.ClusterSpec) error {
	data, err := etcd.ToJson(spec)
	if err != nil {
		return fmt.Errorf("error serializing cluster spec: %v", err)
	}

	p := s.commandsBase.Join(EtcdClusterSpec)
	if err := p.WriteFile(bytes.NewReader([]byte(data)), nil); err != nil {
		return fmt.Errorf("error writing cluster spec file %s: %v", p.Path(), err)
	}

	return nil
}

func (s *vfsStore) IsNewCluster() (bool, error) {
	p := s.commandsBase.Join(EtcdClusterCreated)

	// Note that we use ReadFile so that we use a GET on S3, which is more consistent
	data, err := p.ReadFile()
	if err != nil {
		if os.IsNotExist(err) {
			// TODO: On S3, loop to try to establish consistency?
			return true, nil
		}
		return false, fmt.Errorf("error reading cluster-creation marker file %s: %v", p.Path(), err)
	}
	if len(data) == 0 {
		return false, fmt.Errorf("marker file %s was unexpectedly empty: %v", p.Path(), err)
	}
	return false, nil
}

type etcdClusterCreated struct {
	Timestamp int64 `json:"timestamp,omitempty"`
}

func (s *vfsStore) MarkClusterCreated() error {
	d := &etcdClusterCreated{
		Timestamp: time.Now().UnixNano(),
	}

	data, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("error serializing cluster-creation marker file: %v", err)
	}

	p := s.commandsBase.Join(EtcdClusterCreated)
	if err := p.WriteFile(bytes.NewReader([]byte(data)), nil); err != nil {
		return fmt.Errorf("error creating cluster-creation marker file %s: %v", p.Path(), err)
	}
	return nil
}
