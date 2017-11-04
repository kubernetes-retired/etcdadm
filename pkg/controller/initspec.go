package controller

import (
	"github.com/golang/glog"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
)

type InitialClusterSpecProvider interface {
	// Returns the initial cluster spec, if one cannot be found in local state or a backup
	GetInitialClusterSpec() (*protoetcd.ClusterSpec, error)
}

func StaticInitialClusterSpecProvider(spec *protoetcd.ClusterSpec) InitialClusterSpecProvider {
	return &staticInitialClusterSpecProvider{
		spec: spec,
	}
}

type staticInitialClusterSpecProvider struct {
	spec *protoetcd.ClusterSpec
}

var _ InitialClusterSpecProvider = &staticInitialClusterSpecProvider{}

func (p *staticInitialClusterSpecProvider) GetInitialClusterSpec() (*protoetcd.ClusterSpec, error) {
	glog.Infof("returning static initial ClusterSpec: %v", p.spec)
	return p.spec, nil
}
