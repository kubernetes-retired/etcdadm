/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package etcd

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"k8s.io/klog/v2"
	protoetcd "sigs.k8s.io/etcdadm/etcd-manager/pkg/apis/etcd"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/backup"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/contextutil"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/dns"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/etcdversions"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/legacy"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/pki"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/privateapi"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/urls"
)

const PreparedValidity = time.Minute

type EtcdServer struct {
	baseDir               string
	peerServer            *privateapi.Server
	etcdNodeConfiguration *protoetcd.EtcdNode
	clusterName           string
	dnsProvider           dns.Provider

	mutex sync.Mutex

	state    *protoetcd.EtcdState
	prepared *preparedState
	process  *etcdProcess

	// listenAddress is the address we configure etcd to bind to
	listenAddress string

	etcdClientsCA *pki.CA
	etcdPeersCA   *pki.CA

	// listenMetricsURLs is the set of URLs where etcd should listen for metrics
	listenMetricsURLs []string

	// peerClientIPs is the set of IPs from which we connect to peers, used for the deeper cert validation in etcd 3.2
	// (see https://github.com/kopeio/etcd-manager/issues/371)
	peerClientIPs []net.IP
}

type preparedState struct {
	validUntil   time.Time
	clusterToken string
}

func NewEtcdServer(baseDir string, clusterName string, listenAddress string, listenMetricsURLs []string, etcdNodeConfiguration *protoetcd.EtcdNode, peerServer *privateapi.Server, dnsProvider dns.Provider, etcdClientsCA *pki.CA, etcdPeersCA *pki.CA, peerClientIPs []net.IP) (*EtcdServer, error) {
	s := &EtcdServer{
		baseDir:               baseDir,
		clusterName:           clusterName,
		listenAddress:         listenAddress,
		peerServer:            peerServer,
		dnsProvider:           dnsProvider,
		etcdNodeConfiguration: etcdNodeConfiguration,
		etcdClientsCA:         etcdClientsCA,
		etcdPeersCA:           etcdPeersCA,
		listenMetricsURLs:     listenMetricsURLs,
		peerClientIPs:         peerClientIPs,
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
			klog.Warningf("error running etcd: %v", err)
		}
	})
}

func readState(baseDir string) (*protoetcd.EtcdState, error) {
	p := filepath.Join(baseDir, "state")
	b, err := os.ReadFile(p)
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

	if err := os.WriteFile(p, b, 0755); err != nil {
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
		exitState, exitError := s.process.ExitState()
		if exitError != nil || exitState != nil {
			klog.Warningf("etcd process exited (error=%v, state=%v)", exitError, exitState)

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
					ip, _, err := net.SplitHostPort(a)
					if err != nil {
						return nil, fmt.Errorf("failed to parse address %s: %v", a, err)
					}
					addressToHosts[ip] = append(addressToHosts[ip], m.Dns)
				}
			}
		}

		if len(addressToHosts) != 0 {
			klog.V(3).Infof("updating hosts: %v", addressToHosts)
			if err := s.dnsProvider.UpdateHosts(addressToHosts); err != nil {
				klog.Warningf("error updating hosts: %v", err)
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

	if request.Phase == protoetcd.Phase_PHASE_CANCEL_PREPARE {
		response := &protoetcd.JoinClusterResponse{}
		if s.prepared == nil {
			klog.Infof("cancelled prepare (already expired): %v", request)
			return response, nil
		}
		if s.prepared.clusterToken == request.ClusterToken {
			s.prepared = nil
			klog.Infof("cancelled prepare in response to request: %v", request)
			return response, nil
		}
		return nil, fmt.Errorf("concurrent prepare in progress %q", s.prepared.clusterToken)
	}

	if s.prepared != nil && time.Now().After(s.prepared.validUntil) {
		klog.Infof("preparation %q expired", s.prepared.clusterToken)
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

	klog.Infof("Reconfigure request: %v", request)

	if err := s.validateHeader(request.Header); err != nil {
		return nil, err
	}

	if s.state == nil {
		return nil, fmt.Errorf("cluster not running")
	}

	response := &protoetcd.ReconfigureResponse{}

	state := proto.Clone(s.state).(*protoetcd.EtcdState)
	meNode, err := s.findSelfNode(state)
	if err != nil {
		return nil, err
	}
	if meNode == nil {
		return nil, fmt.Errorf("could not find self node in cluster: %v", err)
	}

	//// We just need to restart to update clienturls
	//if len(request.ClientUrls) != 0 {
	//	me.ClientUrls = request.ClientUrls
	//}

	if request.SetEtcdVersion != "" {
		_, err := BindirForEtcdVersion(request.SetEtcdVersion, "etcd")
		if err != nil {
			return nil, fmt.Errorf("etcd version %q not supported", request.SetEtcdVersion)
		}

		state.EtcdVersion = request.SetEtcdVersion
	}

	state.Quarantined = request.Quarantined

	if request.EnableTls {
		meNode.TlsEnabled = true
		meNode.PeerUrls = urls.RewriteScheme(meNode.PeerUrls, "http://", "https://")
		meNode.ClientUrls = urls.RewriteScheme(meNode.ClientUrls, "http://", "https://")
	}

	klog.Infof("Stopping etcd for reconfigure request: %v", request)
	_, err = s.stopEtcdProcess()
	if err != nil {
		return nil, fmt.Errorf("error stoppping etcd process: %v", err)
	}

	s.state = state
	klog.Infof("updated cluster state: %s", proto.CompactTextString(state))
	if err := writeState(s.baseDir, s.state); err != nil {
		return nil, err
	}

	klog.Infof("Starting etcd version %q", s.state.EtcdVersion)
	if err := s.startEtcdProcess(s.state); err != nil {
		return nil, err
	}

	return response, nil
}

// StopEtcd requests the the node stop running etcd
func (s *EtcdServer) StopEtcd(ctx context.Context, request *protoetcd.StopEtcdRequest) (*protoetcd.StopEtcdResponse, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	klog.Infof("StopEtcd request: %v", request)

	if err := s.validateHeader(request.Header); err != nil {
		return nil, err
	}

	if s.state == nil {
		return nil, fmt.Errorf("cluster not running")
	}

	clusterToken := s.state.Cluster.ClusterToken

	response := &protoetcd.StopEtcdResponse{}

	klog.Infof("Stopping etcd for stop request: %v", request)
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
		klog.Warningf("error creating trashcan directory %s: %v", trashcanDir, err)
	}

	newDataDir := filepath.Join(trashcanDir, clusterToken)
	klog.Infof("archiving etcd data directory %s -> %s", oldDataDir, newDataDir)

	if err := os.Rename(oldDataDir, newDataDir); err != nil {
		klog.Warningf("error renaming directory %s -> %s: %v", oldDataDir, newDataDir, err)
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
		return nil, fmt.Errorf("request Storage is required")
	}
	if request.Info == nil {
		return nil, fmt.Errorf("request Info is required")
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
				klog.Infof("Nodes: %v", state.Cluster.Nodes)
				return nil, fmt.Errorf("multiple nodes matching local name %s included in cluster", node.Name)
			}
			meNode = node
		}
	}
	if meNode == nil {
		klog.Infof("unable to find node in cluster")
		klog.Infof("self node: %v", s.etcdNodeConfiguration)
		klog.Infof("cluster: %v", state.Cluster.Nodes)
	}
	return meNode, nil
}

func (s *EtcdServer) startEtcdProcess(state *protoetcd.EtcdState) error {
	klog.Infof("starting etcd with state %v", state)
	if state.Cluster == nil {
		return fmt.Errorf("cluster not configured, cannot start etcd")
	}
	if state.Cluster.ClusterToken == "" {
		return fmt.Errorf("ClusterToken not configured, cannot start etcd")
	}
	dataDir := filepath.Join(s.baseDir, "data", state.Cluster.ClusterToken)
	pkiDir := filepath.Join(s.baseDir, "pki", state.Cluster.ClusterToken)
	klog.Infof("starting etcd with datadir %s", dataDir)

	state = proto.Clone(state).(*protoetcd.EtcdState)

	// TODO: Validate this during the PREPARE phase
	meNode, err := s.findSelfNode(state)
	if err != nil {
		return err
	}
	if meNode == nil {
		return fmt.Errorf("self node was not included in cluster")
	}

	if !reflect.DeepEqual(s.etcdNodeConfiguration.ClientUrls, meNode.ClientUrls) {
		klog.Infof("overriding clientURLs with %v (state had %v)", s.etcdNodeConfiguration.ClientUrls, meNode.ClientUrls)
		meNode.ClientUrls = s.etcdNodeConfiguration.ClientUrls
	}
	if !reflect.DeepEqual(s.etcdNodeConfiguration.QuarantinedClientUrls, meNode.QuarantinedClientUrls) {
		klog.Infof("overriding quarantinedClientURLs with %v (state had %v)", s.etcdNodeConfiguration.QuarantinedClientUrls, meNode.QuarantinedClientUrls)
		meNode.QuarantinedClientUrls = s.etcdNodeConfiguration.QuarantinedClientUrls
	}

	p := &etcdProcess{
		CreateNewCluster: false,
		DataDir:          dataDir,
		Cluster: &protoetcd.EtcdCluster{
			ClusterToken: state.Cluster.ClusterToken,
			Nodes:        state.Cluster.Nodes,
		},
		Quarantined:       state.Quarantined,
		MyNodeName:        s.etcdNodeConfiguration.Name,
		ListenAddress:     s.listenAddress,
		DisableTLS:        !meNode.TlsEnabled,
		ListenMetricsURLs: s.listenMetricsURLs,
	}

	// We always generate the keypairs, as this allows us to switch to TLS without a restart
	if err := p.createKeypairs(s.etcdPeersCA, s.etcdClientsCA, pkiDir, meNode, s.peerClientIPs); err != nil {
		return err
	}

	etcdVersion := state.EtcdVersion
	// Use the recommended etcd version
	{
		startWith := etcdversions.EtcdVersionForAdoption(etcdVersion)
		if startWith != "" && startWith != etcdVersion {
			klog.Warningf("starting server from etcd %q, will start with %q", etcdVersion, startWith)
			etcdVersion = startWith
		}
	}
	p.EtcdVersion = etcdVersion

	binDir, err := BindirForEtcdVersion(etcdVersion, "etcd")
	if err != nil {
		return err
	}
	p.BinDir = binDir

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

	klog.Infof("killing etcd with datadir %s", s.process.DataDir)
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
		klog.Infof("request had incorrect ClusterName. ClusterName=%q but header=%q", s.clusterName, header)
		return fmt.Errorf("ClusterName mismatch")
	}

	// TODO: Validate (our) peer id?

	if !s.peerServer.IsLeader(header.LeadershipToken) {
		return fmt.Errorf("LeadershipToken in request %q is not current leader", header.LeadershipToken)
	}

	return nil
}
