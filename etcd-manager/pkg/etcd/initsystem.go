package etcd

import (
	"crypto/tls"

	"sigs.k8s.io/etcdadm/apis"
	protoetcd "sigs.k8s.io/etcdadm/etcd-manager/pkg/apis/etcd"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/backup"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/etcdclient"
	"sigs.k8s.io/etcdadm/initsystem"
)

type initSystemEtcdProcess struct {
	initSystem          initsystem.InitSystem
	config              apis.EtcdAdmConfig
	etcdClientTLSConfig *tls.Config

	// baseDir is the directory in which we launch the binary (cwd)
	baseDir string
}

// DataDir implements EtcdProcess
func (p *initSystemEtcdProcess) DataDir() string {
	return p.config.DataDir
}

// EtcdVersion implements EtcdProcess
func (p *initSystemEtcdProcess) EtcdVersion() string {
	return p.config.Version
}

// IsActive returns true if we are running
func (p *initSystemEtcdProcess) IsActive() (bool, error) {
	return p.initSystem.IsActive()
}

// DoBackup performs a backup/snapshot of the data
func (p *initSystemEtcdProcess) DoBackup(store backup.Store, info *protoetcd.BackupInfo) (*protoetcd.DoBackupResponse, error) {
	// me := p.findMyNode()
	// if me == nil {
	// 	return nil, fmt.Errorf("unable to find self node %q in %v", p.MyNodeName, p.Cluster.Nodes)
	// }

	// clientUrls := me.ClientUrls
	// if p.Quarantined {
	// 	clientUrls = me.QuarantinedClientUrls
	// }

	var clientURLs []string
	for _, u := range p.config.AdvertiseClientURLs {
		clientURLs = append(clientURLs, u.String())
	}
	return DoBackup(store, info, p.config.DataDir, clientURLs, p.etcdClientTLSConfig)
}

func (p *initSystemEtcdProcess) NewClient() (*etcdclient.EtcdClient, error) {
	// var me *protoetcd.EtcdNode
	// for _, node := range p.Cluster.Nodes {
	// 	if node.Name == p.MyNodeName {
	// 		me = node
	// 	}
	// }
	// if me == nil {
	// 	return nil, fmt.Errorf("unable to find self node %q in %v", p.MyNodeName, p.Cluster.Nodes)
	// }

	// clientURLs := me.ClientUrls
	// if p.Quarantined {
	// 	clientURLs = me.QuarantinedClientUrls
	// }

	var clientURLs []string
	for _, u := range p.config.AdvertiseClientURLs {
		clientURLs = append(clientURLs, u.String())
	}
	return NewClient(clientURLs, p.baseDir, p.etcdClientTLSConfig)
}

func (p *initSystemEtcdProcess) Stop() error {
	if p.initSystem == nil {
		return nil
	}
	if err := p.initSystem.DisableAndStopService(); err != nil {
		return err
	}
	return nil
}
