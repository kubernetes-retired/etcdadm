package backup

import (
	"k8s.io/kops/util/pkg/vfs"
	"kope.io/etcd-manager/pkg/apis/etcd"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
)

const MetaFilename = "_etcd_backup.meta"
const DataFilename = "etcd.backup.gz"

const CommandFilename = "_command.json"

type Store interface {
	Spec() string

	// AddBackup adds a backup to the store, returning the name of the backup
	AddBackup(backupFile string, sequence string, info *etcd.BackupInfo) (string, error)

	// ListBackups returns all the available backups, in chronological order
	ListBackups() ([]string, error)

	// RemoveBackup deletes a backup (as returned by ListBackups)
	RemoveBackup(backup string) error

	// LoadInfo loads the backup information that should have been saved alongside a backup
	LoadInfo(backup string) (*etcd.BackupInfo, error)

	// DownloadBackup downloads the backup to the specific file
	DownloadBackup(name string, destFile string) error

	// AddCommand adds a command to the back of the queue
	AddCommand(cmd *protoetcd.Command) error

	// ListCommands returns all the external commands that have not been removed
	ListCommands() ([]*Command, error)

	// RemoveCommand marks a command as complete
	RemoveCommand(command *Command) error
}

type Command struct {
	p    vfs.Path
	Data protoetcd.Command
}

func NewStore(storage string) (Store, error) {
	//u, err := url.Parse(storage)
	//if err != nil {
	//	return nil, fmt.Errorf("error parsing storage url %q", storage)
	//}
	//
	//switch u.Scheme {
	//case "file":
	//	return NewFilesystemStore(u)
	//
	//default:
	//	return nil, fmt.Errorf("unknown storage scheme %q", storage)
	//}

	p, err := vfs.Context.BuildVfsPath(storage)
	if err != nil {
		return nil, err
	}
	return NewVFSStore(p)
}
