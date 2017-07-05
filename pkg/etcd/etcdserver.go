package etcd

import (
	"fmt"
	"sync"

	"golang.org/x/net/context"
	"kope.io/etcd-manager/pkg/privateapi"
)

type EtcdServer struct {
	peerServer *privateapi.Server

	mutex        sync.Mutex
	etcdClusters map[string]*EtcdManager
}

func NewEtcdServer(peerServer *privateapi.Server) *EtcdServer {
	s := &EtcdServer{
		peerServer: peerServer,

		etcdClusters: make(map[string]*EtcdManager),
	}

	RegisterEtcdManagerServiceServer(peerServer.GrpcServer(), s)
	return s
}

var _ EtcdManagerServiceServer = &EtcdServer{}

// GetInfo gets info about the node
func (s *EtcdServer) GetInfo(context.Context, *GetInfoRequest) (*GetInfoResponse, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	response := &GetInfoResponse{}
	for _, etcdCluster := range s.etcdClusters {
		pb := &EtcdCluster{}
		*pb = etcdCluster.model
		response.Clusters = append(response.Clusters, pb)
	}

	return response, nil
}

// JoinCluster requests that the node join an existing cluster
func (s *EtcdServer) JoinCluster(ctx context.Context, request *JoinClusterRequest) (*JoinClusterResponse, error) {
	if request.Cluster == nil {
		return nil, fmt.Errorf("Cluster is required")
	}
	name := request.Cluster.ClusterName
	if name == "" {
		return nil, fmt.Errorf("ClusterName is required")
	}

	manager := s.findManager(name)
	if manager == nil {
		// We could in theory automatically add clusters, but I don't think it's needed
		return nil, fmt.Errorf("Cluster %q not registered", name)
	}

	return manager.joinCluster(ctx, request)
}

func (s *EtcdServer) findManager(name string) *EtcdManager {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.etcdClusters[name]
}
