package etcd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/pkg/backup"
	"kope.io/etcd-manager/pkg/contextutil"
	"kope.io/etcd-manager/pkg/hosts"
	"kope.io/etcd-manager/pkg/legacy"
	"kope.io/etcd-manager/pkg/privateapi"
)

const PreparedValidity = time.Minute

type EtcdServer struct {
	baseDir               string
	peerServer            *privateapi.Server
	etcdNodeConfiguration *protoetcd.EtcdNode
	clusterName           string

	backupStore backup.Store

	mutex sync.Mutex

	state    *protoetcd.EtcdState
	prepared *preparedState
	process  *etcdProcess

	// listenAddress is the address we configure etcd to bind to
	listenAddress string
}

type preparedState struct {
	validUntil   time.Time
	clusterToken string
}

func NewEtcdServer(baseDir string, clusterName string, listenAddress string, etcdNodeConfiguration *protoetcd.EtcdNode, peerServer *privateapi.Server) (*EtcdServer, error) {
	s := &EtcdServer{
		baseDir:               baseDir,
		clusterName:           clusterName,
		listenAddress:         listenAddress,
		peerServer:            peerServer,
		etcdNodeConfiguration: etcdNodeConfiguration,
	}

	// Make sure we have read state from disk before serving
	if err := s.initState(); err != nil {
		return nil, err
	}

	if s.state == nil {
		if state, err := legacy.ImportExistingEtcd(baseDir, etcdNodeConfiguration); err != nil {
			return nil, err
		} else if state != nil {
			if err := writeState(s.baseDir, state); err != nil {
				return nil, err
			}
			s.state = state
		}
	}

	protoetcd.RegisterEtcdManagerServiceServer(peerServer.GrpcServer(), s)
	return s, nil
}

var _ protoetcd.EtcdManagerServiceServer = &EtcdServer{}

func (s *EtcdServer) Run(ctx context.Context) {
	contextutil.Forever(ctx, time.Second*10, func() {
		err := s.runOnce()
		if err != nil {
			glog.Warningf("error running etcd: %v", err)
		}
	})
}

func readState(baseDir string) (*protoetcd.EtcdState, error) {
	p := filepath.Join(baseDir, "state")
	b, err := ioutil.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error reading state file %q: %v", p, err)
	}

	state := &protoetcd.EtcdState{}
	if err := proto.Unmarshal(b, state); err != nil {
		// TODO: Have multiple state files?
		return nil, fmt.Errorf("error parsing state file: %v", err)
	}

	return state, nil
}

func writeState(baseDir string, state *protoetcd.EtcdState) error {
	p := filepath.Join(baseDir, "state")

	b, err := proto.Marshal(state)
	if err != nil {
		return fmt.Errorf("error marshaling state data: %v", err)
	}

	if err := ioutil.WriteFile(p, b, 0755); err != nil {
		return fmt.Errorf("error writing state file %q: %v", p, err)
	}
	return nil
}

func (s *EtcdServer) initState() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.state == nil {
		state, err := readState(s.baseDir)
		if err != nil {
			return err
		}

		if state != nil {
			s.state = state
		}
	}

	return nil
}

func (s *EtcdServer) runOnce() error {
	if err := s.initState(); err != nil {
		return err
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Check that etcd process is still running
	if s.process != nil {
		exitError, exitState := s.process.ExitState()
		if exitError != nil || exitState != nil {
			glog.Warningf("etcd process exited (error=%v, state=%v)", exitError, exitState)

			s.process = nil
		}
	}

	// Start etcd, if it is not running but should be
	if s.state != nil && s.state.Cluster != nil && s.process == nil {
		if err := s.startEtcdProcess(s.state); err != nil {
			return err
		}
	}

	return nil
}

// GetInfo gets info about the node
func (s *EtcdServer) GetInfo(ctx context.Context, request *protoetcd.GetInfoRequest) (*protoetcd.GetInfoResponse, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	response := &protoetcd.GetInfoResponse{}
	response.ClusterName = s.clusterName
	response.NodeConfiguration = s.etcdNodeConfiguration

	if s.state != nil && s.state.Cluster != nil {
		response.EtcdState = s.state
	}

	//if s.state != nil && s.state.Cluster != nil {
	//	//pb := &protoetcd.EtcdCluster{}
	//	//*pb = *s.state.Cluster
	//	response.EtcdConfigured = true
	//	response.ClusterToken = s.state.Cluster.ClusterToken
	//	if s.state.Cluster.Me != nil {
	//		response.NodeConfiguration = s.state.Cluster.Me
	//	}
	//	//response.ClusterConfiguration = pb
	//}

	return response, nil
}

// UpdateEndpoints updates /etc/hosts with the other cluster members
func (s *EtcdServer) UpdateEndpoints(ctx context.Context, request *protoetcd.UpdateEndpointsRequest) (*protoetcd.UpdateEndpointsResponse, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	response := &protoetcd.UpdateEndpointsResponse{}

	if request.MemberMap != nil {
		addressToHosts := make(map[string][]string)

		for _, m := range request.MemberMap.Members {
			if m.Dns != "" {
				for _, a := range m.Addresses {
					addressToHosts[a] = append(addressToHosts[a], m.Dns)
				}
			}
		}

		if len(addressToHosts) != 0 {
			glog.Infof("updating hosts: %v", addressToHosts)
			if err := hosts.UpdateHostsFileWithRecords("/etc/hosts", "etcd-manager["+s.clusterName+"]", addressToHosts); err != nil {
				glog.Warningf("error updating hosts: %v", err)
				return nil, err
			}
		}
	}

	return response, nil
}

// JoinCluster requests that the node join an existing cluster
func (s *EtcdServer) JoinCluster(ctx context.Context, request *protoetcd.JoinClusterRequest) (*protoetcd.JoinClusterResponse, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if err := s.validateHeader(request.Header); err != nil {
		return nil, err
	}

	if s.prepared != nil && time.Now().After(s.prepared.validUntil) {
		glog.Infof("preparation %q expired", s.prepared.clusterToken)
		s.prepared = nil
	}

	_, err := BindirForEtcdVersion(request.EtcdVersion, "etcd")
	if err != nil {
		return nil, fmt.Errorf("etcd version %q not supported", request.EtcdVersion)
	}

	response := &protoetcd.JoinClusterResponse{}

	switch request.Phase {
	case protoetcd.Phase_PHASE_PREPARE:
		if s.process != nil {
			return nil, fmt.Errorf("etcd process already running")
		}

		if s.prepared != nil {
			return nil, fmt.Errorf("concurrent prepare in progress %q", s.prepared.clusterToken)
		}

		s.prepared = &preparedState{
			validUntil:   time.Now().Add(PreparedValidity),
			clusterToken: request.ClusterToken,
		}
		return response, nil

	case protoetcd.Phase_PHASE_INITIAL_CLUSTER:
		if s.process != nil {
			return nil, fmt.Errorf("etcd process already running")
		}

		if s.prepared == nil {
			return nil, fmt.Errorf("not prepared")
		}
		if s.prepared.clusterToken != request.ClusterToken {
			return nil, fmt.Errorf("clusterToken %q does not match prepared %q", request.ClusterToken, s.prepared.clusterToken)
		}

		if s.state == nil {
			s.state = &protoetcd.EtcdState{}
		}
		s.state.NewCluster = true
		s.state.Cluster = &protoetcd.EtcdCluster{
			ClusterToken: request.ClusterToken,
			Nodes:        request.Nodes,
		}
		s.state.Quarantined = true
		s.state.EtcdVersion = request.EtcdVersion

		if err := writeState(s.baseDir, s.state); err != nil {
			return nil, err
		}

		if err := s.startEtcdProcess(s.state); err != nil {
			return nil, err
		}

		// TODO: Wait for etcd initialization before marking as existing?
		s.state.NewCluster = false
		if err := writeState(s.baseDir, s.state); err != nil {
			return nil, err
		}
		s.prepared = nil
		return response, nil

	case protoetcd.Phase_PHASE_JOIN_EXISTING:
		if s.process != nil {
			return nil, fmt.Errorf("etcd process already running")
		}

		if s.prepared == nil {
			return nil, fmt.Errorf("not prepared")
		}
		if s.prepared.clusterToken != request.ClusterToken {
			return nil, fmt.Errorf("clusterToken %q does not match prepared %q", request.ClusterToken, s.prepared.clusterToken)
		}

		if s.state == nil {
			s.state = &protoetcd.EtcdState{}
		}
		s.state.NewCluster = false
		s.state.Cluster = &protoetcd.EtcdCluster{
			ClusterToken: request.ClusterToken,
			Nodes:        request.Nodes,
		}
		s.state.Quarantined = false
		s.state.EtcdVersion = request.EtcdVersion

		if err := writeState(s.baseDir, s.state); err != nil {
			return nil, err
		}

		if err := s.startEtcdProcess(s.state); err != nil {
			return nil, err
		}
		// TODO: Wait for join?
		s.prepared = nil
		return response, nil

	default:
		return nil, fmt.Errorf("unknown status %s", request.Phase)
	}
}

// Reconfigure requests that the node reconfigure itself, in particular for an etcd upgrade/downgrade
func (s *EtcdServer) Reconfigure(ctx context.Context, request *protoetcd.ReconfigureRequest) (*protoetcd.ReconfigureResponse, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	glog.Infof("Reconfigure request: %v", request)

	if err := s.validateHeader(request.Header); err != nil {
		return nil, err
	}

	if s.state == nil {
		return nil, fmt.Errorf("cluster not running")
	}

	response := &protoetcd.ReconfigureResponse{}

	state := proto.Clone(s.state).(*protoetcd.EtcdState)
	me, err := s.findSelfNode(state)
	if err != nil {
		return nil, err
	}
	if me == nil {
		return nil, fmt.Errorf("could not find self node in cluster: %v", err)
	}

	//// We just need to restart to update clienturls
	//if len(request.ClientUrls) != 0 {
	//	me.ClientUrls = request.ClientUrls
	//}

	if request.EtcdVersion != "" {
		_, err := BindirForEtcdVersion(request.EtcdVersion, "etcd")
		if err != nil {
			return nil, fmt.Errorf("etcd version %q not supported", request.EtcdVersion)
		}

		state.EtcdVersion = request.EtcdVersion
	}

	state.Quarantined = request.Quarantined

	glog.Infof("Stopping etcd for reconfigure request: %v", request)
	_, err = s.stopEtcdProcess()
	if err != nil {
		return nil, fmt.Errorf("error stoppping etcd process: %v", err)
	}

	s.state = state
	if err := writeState(s.baseDir, s.state); err != nil {
		return nil, err
	}

	glog.Infof("Starting etcd version %q", s.state.EtcdVersion)
	if err := s.startEtcdProcess(s.state); err != nil {
		return nil, err
	}

	return response, nil
}

// StopEtcd requests the the node stop running etcd
func (s *EtcdServer) StopEtcd(ctx context.Context, request *protoetcd.StopEtcdRequest) (*protoetcd.StopEtcdResponse, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	glog.Infof("StopEtcd request: %v", request)

	if err := s.validateHeader(request.Header); err != nil {
		return nil, err
	}

	if s.state == nil {
		return nil, fmt.Errorf("cluster not running")
	}

	clusterToken := s.state.Cluster.ClusterToken

	response := &protoetcd.StopEtcdResponse{}

	glog.Infof("Stopping etcd for stop request: %v", request)
	if _, err := s.stopEtcdProcess(); err != nil {
		return nil, fmt.Errorf("error stoppping etcd process: %v", err)
	}

	state := &protoetcd.EtcdState{}
	s.state = state
	if err := writeState(s.baseDir, s.state); err != nil {
		return nil, err
	}

	oldDataDir := filepath.Join(s.baseDir, "data", clusterToken)
	trashcanDir := filepath.Join(s.baseDir, "data-trashcan")
	if err := os.MkdirAll(trashcanDir, 0755); err != nil {
		glog.Warningf("error creating trashcan directory %s: %v", trashcanDir, err)
	}

	newDataDir := filepath.Join(trashcanDir, clusterToken)
	glog.Infof("archiving etcd data directory %s -> %s", oldDataDir, newDataDir)

	if err := os.Rename(oldDataDir, newDataDir); err != nil {
		glog.Warningf("error renaming directory %s -> %s: %v", oldDataDir, newDataDir, err)
	}

	return response, nil
}

// DoBackup performs a backup to the backupstore
func (s *EtcdServer) DoBackup(ctx context.Context, request *protoetcd.DoBackupRequest) (*protoetcd.DoBackupResponse, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if err := s.validateHeader(request.Header); err != nil {
		return nil, err
	}

	if s.process == nil {
		return nil, fmt.Errorf("etcd not running")
	}

	if request.Storage == "" {
		return nil, fmt.Errorf("Storage is required")
	}
	if request.Info == nil {
		return nil, fmt.Errorf("Info is required")
	}
	backupStore, err := backup.NewStore(request.Storage)
	if err != nil {
		return nil, err
	}

	info := request.Info
	info.EtcdVersion = s.process.EtcdVersion

	response, err := s.process.DoBackup(backupStore, info)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (s *EtcdServer) findSelfNode(state *protoetcd.EtcdState) (*protoetcd.EtcdNode, error) {
	var meNode *protoetcd.EtcdNode
	for _, node := range state.Cluster.Nodes {
		if node.Name == s.etcdNodeConfiguration.Name {
			if meNode != nil {
				glog.Infof("Nodes: %v", state.Cluster.Nodes)
				return nil, fmt.Errorf("multiple nodes matching local name %s included in cluster", node.Name)
			}
			meNode = node
		}
	}
	if meNode == nil {
		glog.Infof("unable to find node in cluster")
		glog.Infof("self node: %v", s.etcdNodeConfiguration)
		glog.Infof("cluster: %v", state.Cluster.Nodes)
	}
	return meNode, nil
}

func (s *EtcdServer) startEtcdProcess(state *protoetcd.EtcdState) error {
	if state.Cluster == nil {
		return fmt.Errorf("cluster not configured, cannot start etcd")
	}
	if state.Cluster.ClusterToken == "" {
		return fmt.Errorf("ClusterToken not configured, cannot start etcd")
	}
	dataDir := filepath.Join(s.baseDir, "data", state.Cluster.ClusterToken)
	glog.Infof("starting etcd with datadir %s", dataDir)

	// TODO: Validate this during the PREPARE phase
	meNode, err := s.findSelfNode(state)
	if err != nil {
		return err
	}
	if meNode == nil {
		return fmt.Errorf("self node was not included in cluster")
	}

	p := &etcdProcess{
		CreateNewCluster: false,
		DataDir:          dataDir,
		Cluster: &protoetcd.EtcdCluster{
			ClusterToken: state.Cluster.ClusterToken,
			Nodes:        state.Cluster.Nodes,
		},
		Quarantined:   state.Quarantined,
		MyNodeName:    s.etcdNodeConfiguration.Name,
		ListenAddress: s.listenAddress,
	}

	binDir, err := BindirForEtcdVersion(state.EtcdVersion, "etcd")
	if err != nil {
		return err
	}
	p.BinDir = binDir
	p.EtcdVersion = state.EtcdVersion

	if state.NewCluster {
		p.CreateNewCluster = true
	}

	if err := p.Start(); err != nil {
		return fmt.Errorf("error starting etcd: %v", err)
	}

	s.process = p

	return nil
}

// StopEtcdProcessForTest terminates etcd if it is running; primarily used for testing
func (s *EtcdServer) StopEtcdProcessForTest() (bool, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.stopEtcdProcess()
}

// stopEtcdProcess terminates etcd if it is running.  This version assumes the lock is held (unlike StopEtcdProcess)
func (s *EtcdServer) stopEtcdProcess() (bool, error) {
	if s.process == nil {
		return false, nil
	}

	glog.Infof("killing etcd with datadir %s", s.process.DataDir)
	err := s.process.Stop()
	if err != nil {
		return true, fmt.Errorf("error killing etcd: %v", err)
	}
	s.process = nil
	return true, nil
}

// validateHeader checks the values in the CommonRequestHeader
func (s *EtcdServer) validateHeader(header *protoetcd.CommonRequestHeader) error {
	if header.ClusterName != s.clusterName {
		glog.Infof("request had incorrect ClusterName.  ClusterName=%q but header=%q", s.clusterName, header)
		return fmt.Errorf("ClusterName mismatch")
	}

	// TODO: Validate (our) peer id?

	if !s.peerServer.IsLeader(header.LeadershipToken) {
		return fmt.Errorf("LeadershipToken in request %q is not current leader", header.LeadershipToken)
	}

	return nil
}
