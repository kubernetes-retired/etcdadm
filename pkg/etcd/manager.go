package etcd

//import (
//	crypto_rand "crypto/rand"
//	"encoding/base64"
//	"fmt"
//	"io"
//	"path/filepath"
//	"sort"
//	"sync"
//	"time"
//	"io/ioutil"
//	"os"
//
//	"github.com/golang/glog"
//	"golang.org/x/net/context"
//	"kope.io/etcd-manager/pkg/privateapi"
//protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
//)
//
//type EtcdController struct {
//	mutex sync.Mutex
//
//	clusterName string
//
//	model   EtcdCluster
//	baseDir string
//
//	process *etcdProcess
//
//	me    privateapi.PeerId
//	peers privateapi.Peers
//}
//
//type etcdClusterState struct {
//	clusterName string
//	members map[string]*etcdMember
//}
//
//type etcdMember struct {
//	node *EtcdNode
//	peer *privateapi.PeerInfo
//}
//
//func (s *EtcdController) NewEtcdManager(model *EtcdCluster, baseDir string) (*EtcdController, error) {
//	s.mutex.Lock()
//	defer s.mutex.Unlock()
//
//	if model.DesiredClusterSize < 1 {
//		return nil, fmt.Errorf("DesiredClusterSize must be > 1")
//	}
//	if model.ClusterName == "" {
//		return nil, fmt.Errorf("ClusterName is required")
//	}
//	if s.etcdClusters[model.ClusterName] != nil {
//		return nil, fmt.Errorf("cluster %q already registered", model.ClusterName)
//	}
//	m := &EtcdController{
//		model:   *model,
//		baseDir: baseDir,
//		peers:   s.peerServer,
//		me:      s.peerServer.MyPeerId(),
//	}
//	s.etcdClusters[model.ClusterName] = m
//	return m, nil
//}
//
//func (m *EtcdController) Run() {
//	for {
//		ctx := context.Background()
//		progress, err := m.run(ctx)
//		if err != nil {
//			glog.Warningf("unexpected error running etcd cluster reconciliation loop: %v", err)
//		}
//		if !progress {
//			time.Sleep(10 * time.Second)
//		}
//	}
//}
//
//func buildEtcdNodeName(clusterToken string, peerId string) string {
//	return clusterToken + "--" + peerId
//}
//
//func (m *EtcdController) run(ctx context.Context) (bool, error) {
//	m.mutex.Lock()
//	defer m.mutex.Unlock()
//
//	desiredMemberCount := int(m.model.DesiredClusterSize)
//
//	peers := m.peers.Peers()
//	sort.SliceStable(peers, func(i, j int) bool {
//		return peers[i].Id < peers[j].Id
//	})
//	glog.Infof("peers: %s", peers)
//
//	var me *privateapi.PeerInfo
//	for _, peer := range peers {
//		if peer.Id == string(m.me) {
//			me = peer
//		}
//	}
//	if me == nil {
//		return false, fmt.Errorf("cannot find self %q in list of peers %s", m.me, peers)
//	}
//
//	if m.process == nil {
//		return m.stepStartCluster(me, peers)
//	}
//
//	if m.process == nil {
//		return false, fmt.Errorf("unexpected state in control loop - no etcd process")
//	}
//
//	members, err := m.process.listMembers()
//	if err != nil {
//		return false, fmt.Errorf("error querying members from etcd: %v", err)
//	}
//
//	// TODO: Only if we are leader
//
//	actualMemberCount := len(members)
//	if actualMemberCount < desiredMemberCount {
//		glog.Infof("will try to add %d members; existing members: %s", desiredMemberCount-actualMemberCount, members)
//
//		// TODO: Randomize peer selection?
//
//		membersByName := make(map[string]*etcdProcessMember)
//		for _, member := range members {
//			membersByName[member.Name] = member
//		}
//
//		for _, peer := range peers {
//			peerMemberName := buildEtcdNodeName(m.process.Cluster.ClusterToken, peer.Id)
//			if membersByName[peerMemberName] != nil {
//				continue
//			}
//
//			cluster := &EtcdCluster{
//				ClusterToken: m.process.Cluster.ClusterToken,
//				ClusterName:  m.process.Cluster.ClusterName,
//			}
//			for _, member := range members {
//				etcdNode := &EtcdNode{
//					Name:       member.Name,
//					PeerUrls:   member.PeerURLs,
//					ClientUrls: member.ClientURLs,
//				}
//				cluster.Nodes = append(cluster.Nodes, etcdNode)
//			}
//			joinClusterRequest := &JoinClusterRequest{
//				Cluster: cluster,
//			}
//			peerGrpcClient, err := m.peers.GetPeerClient(privateapi.PeerId(peer.Id))
//			if err != nil {
//				return false, fmt.Errorf("error getting peer client %q: %v", peer.Id, err)
//			}
//			peerClient := NewEtcdManagerServiceClient(peerGrpcClient)
//			joinClusterResponse, err := peerClient.JoinCluster(ctx, joinClusterRequest)
//			if err != nil {
//				return false, fmt.Errorf("error from JoinClusterRequest from peer %q: %v", peer.Id, err)
//			}
//			if joinClusterResponse.Node == nil {
//				return false, fmt.Errorf("JoingClusterResponse from peer %q did not include peer info", peer.Id, joinClusterResponse)
//			}
//			peerURLs := joinClusterResponse.Node.PeerUrls
//			//var peerURLs []string
//			//for _, address := range peer.Addresses {
//			//	peerURL := fmt.Sprintf("http://%s:%d", address, m.model.PeerPort)
//			//	peerURLs = append(peerURLs, peerURL)
//			//}
//			glog.Infof("will try to add member %s @ %s", peerMemberName, peerURLs)
//			member, err := m.process.addMember(peerMemberName, peerURLs)
//			if err != nil {
//				return false, fmt.Errorf("failed to add peer %s %s: %v", peerMemberName, peerURLs, err)
//			}
//			glog.Infof("Added member: %s", member)
//			actualMemberCount++
//			if actualMemberCount == desiredMemberCount {
//				break
//			}
//		}
//	}
//
//	// TODO: Eventually replace unhealthy members?
//
//	return false, nil
//}
//
//func randomToken() string {
//	b := make([]byte, 16, 16)
//	_, err := io.ReadFull(crypto_rand.Reader, b)
//	if err != nil {
//		glog.Fatalf("error generating random token: %v", err)
//	}
//	return base64.RawURLEncoding.EncodeToString(b)
//}
//
//func (m*EtcdController) updateClusterState(peers []*privateapi.PeerInfo) {
//	clusterStates := make(map[string]*etcdClusterState)
//
//	// Collect info from each peer
//	for _, peer := range peers {
//		getInfoRequest := &GetInfoRequest{
//		}
//
//		getInfoResponse, err := m.rpcGetInfo(peer.Id, ctx, getInfoRequest)
//		if err != nil {
//			return false, fmt.Errorf("error from GetInfo from peer %q: %v", peer.Id, err)
//		}
//
//		for _, cluster := range getInfoResponse.Clusters {
//			clusterState := clusterStates[cluster.ClusterName]
//			if clusterState == nil {
//				clusterState = &etcdClusterState{
//					clusterName: cluster.ClusterName,
//					members: make(map[string]*etcdMember),
//				}
//				clusterStates[cluster.ClusterName] = clusterState
//			}
//
//			for _, node := range cluster.Nodes {
//				// TODO: Only for the local node
//				if len(cluster.Nodes) != 1 {
//					panic("multinodes not implemented")
//				}
//				etcdMember := clusterState.members[node.Name]
//				if etcdMember == nil {
//					etcdMember = &etcdMember{
//						node: node,
//						peer: peer,
//					}
//					clusterState.members[node.Name] = etcdMember
//				}
//			}
//		}
//	}
//
//	// Query each cluster for its state
//	for cluster, clusterState := range clusterStates {
//		found := false
//		for _, node := range nodes {
//			members, err := etcdListMembers(node)
//			if err != nil {
//				glog.Warningf("unable to reach member %s: %v", node, err)
//			}
//			if err == nil {
//				found = true
//				break
//			}
//		}
//	}
//
//	return clusterStates
//}
//
//func (m *EtcdController) addNodeToCluster(clusterState *clusterState, newMember *etcdMember) (*JoinClusterResponse, error) {
//	members, err := m.process.listMembers()
//	if err != nil {
//		// TODO: We probably want to retry here, as if we have an unhealthy cluster we want to kill it and join a better one
//		return nil, fmt.Errorf("error querying members from etcd: %v", err)
//	}
//
//	var proposal []*privateapi.PeerInfo
//
//	// Try to prepare an add on the new node and an existing cluster member (any member)
//	{
//		joinClusterRequest := &JoinClusterRequest{
//			Phase: Phase_PHASE_PREPARE,
//			ClusterName: clusterState.clusterName,
//			ClusterToken: clusterState.clusterToken,
//			Nodes: clusterState.nodes,
//		}
//
//		joinClusterResponse, err := m.rpcJoinCluster(newMember.peer.Id, ctx, joinClusterRequest)
//		if err != nil {
//			return false, fmt.Errorf("error from JoinClusterRequest from peer %q: %v", peer.Id, err)
//		}
//	}
//	for _, member := range clusterState.members {
//		joinClusterRequest := &JoinClusterRequest{
//			Phase: Phase_PHASE_PREPARE,
//			ClusterName: clusterState.clusterName,
//			ClusterToken: clusterState.clusterToken,
//			Nodes: clusterState.nodes,
//		}
//
//		joinClusterResponse, err := m.rpcJoinCluster(member.peer.Id, ctx, joinClusterRequest)
//		if err != nil {
//			// TODO: Try other members?
//			return false, fmt.Errorf("error from JoinClusterRequest from peer %q: %v", peer.Id, err)
//		}
//		break
//	}
//
//	// Do the actual add
//	{
//		joinClusterRequest := &JoinClusterRequest{
//			Phase: Phase_PHASE_JOIN_EXISTING,
//			ClusterName: clusterState.clusterName,
//			ClusterToken: clusterState.clusterToken,
//			Nodes: clusterState.nodes,
//		}
//
//		joinClusterResponse, err := m.rpcJoinCluster(newMember.peer.Id, ctx, joinClusterRequest)
//		if err != nil {
//			return false, fmt.Errorf("error from JoinClusterRequest from peer %q: %v", peer.Id, err)
//		}
//	}
//	for _, member := range clusterState.members {
//		joinClusterRequest := &JoinClusterRequest{
//			Phase: Phase_PHASE_JOIN_EXISTING,
//			ClusterName: clusterState.clusterName,
//			ClusterToken: clusterState.clusterToken,
//			Nodes: clusterState.nodes,
//		}
//
//		joinClusterResponse, err := m.rpcJoinCluster(member.peer.Id, ctx, joinClusterRequest)
//		if err != nil {
//			// TODO: Try other members?
//			return false, fmt.Errorf("error from JoinClusterRequest from peer %q: %v", peer.Id, err)
//		}
//		break
//	}
//
//	return nil
//}
//
//func (m *EtcdController) rpcJoinCluster(peer *privateapi.PeerInfo, ctx *context.Context, joinClusterRequest *JoinClusterRequest) (*JoinClusterResponse, error) {
//	peerGrpcClient, err := m.peers.GetPeerClient(privateapi.PeerId(peer.Id))
//	if err != nil {
//		return nil, fmt.Errorf("error getting peer client %q: %v", peer.Id, err)
//	}
//	peerClient := NewEtcdManagerServiceClient(peerGrpcClient)
//	return peerClient.JoinCluster(ctx, joinClusterRequest)
//}
//
//func (m *EtcdController) rpcGetInfo(peer *privateapi.PeerInfo, ctx *context.Context, request *GetInfoRequest) (*GetInfoResponse, error) {
//	peerGrpcClient, err := m.peers.GetPeerClient(privateapi.PeerId(peer.Id))
//	if err != nil {
//		return nil, fmt.Errorf("error getting peer client %q: %v", peer.Id, err)
//	}
//	peerClient := NewEtcdManagerServiceClient(peerGrpcClient)
//	return peerClient.GetInfo(ctx, request)
//}
//
//func (m *EtcdController) stepStartCluster(me *privateapi.PeerInfo, peers []*privateapi.PeerInfo) (bool, error) {
//	desiredMemberCount := int(m.model.DesiredClusterSize)
//	quorumSize := (desiredMemberCount / 2) + 1
//
//	// TODO: Query the others to make sure they are OK with joining?
//
//	// We will start a single member cluster, with us as the node, and then we will try to join others to us
//	var dirs []string
//	files, err := ioutil.ReadDir(m.baseDir)
//	if err != nil {
//		return false, fmt.Errorf("error listing contents of %s: %v", m.baseDir, err)
//	}
//	for _, f := range files {
//		name := f.Name()
//		if f.IsDir() {
//			dirs = append(dirs, name)
//		}
//	}
//
//	if len(dirs) > 1 {
//		// If there are multiple dirs see if we can whittle the list using the state file
//		var activeDirs []string
//		for _, dir := range dirs {
//			p := filepath.Join(m.baseDir, dir+".etcdmanager")
//			_, err := os.Stat(p)
//			if err != nil {
//				if !os.IsNotExist(err) {
//					return false, fmt.Errorf("error checking for stat of %s: %v", p, err)
//				}
//			} else {
//				activeDirs = append(activeDirs)
//			}
//		}
//		if len(activeDirs) != 0 {
//			dirs = activeDirs
//		}
//	}
//
//	var clusterToken string
//	createNewCluster := false
//
//	if len(dirs) > 1 {
//		return false, fmt.Errorf("unable to determine active cluster version from %s", dirs)
//	} else if len(dirs) == 1 {
//		// Existing cluster
//		clusterToken = dirs[0]
//	} else {
//		// Consider starting a new cluster?
//
//		if len(peers) < quorumSize {
//			glog.Infof("Insufficient peers to form a quorum %d, won't proceed", quorumSize)
//			return false, nil
//		}
//
//		if peers[0].Id != me.Id {
//			// We are not the leader, we won't initiate
//			glog.Infof("we are not leader, won't initiate cluster creation")
//			return false, nil
//		}
//
//		createNewCluster = true
//		clusterToken = randomToken()
//	}
//
//	dataDir := filepath.Join(m.baseDir, clusterToken)
//
//	{
//		p := filepath.Join(m.baseDir, clusterToken+".etcdmanager")
//		if err := ioutil.WriteFile(p, []byte{}, 0755); err != nil {
//			return false, fmt.Errorf("error writing state file %s: %q", p)
//		}
//	}
//
//	var clusterInfo *clusterInfo
//
//	meNode := clusterInfo.buildEtcdNode(me)
//
//	ctx := context.Background()
//
//	var proposal []*privateapi.PeerInfo
//	for _, peer := range peers {
//		// Note the we send the message to ourselves
//		joinClusterRequest := &JoinClusterRequest{
//			Phase: Phase_PHASE_PREPARE,
//			ClusterName: m.clusterName,
//			ClusterToken: clusterToken,
//			Nodes: nodes,
//		}
//
//		joinClusterResponse, err := m.rpcJoinCluster(peer.Id, ctx, joinClusterRequest)
//		if err != nil {
//			// TODO: Send a CANCEL message for anything PREPAREd?
//			return false, fmt.Errorf("error from JoinClusterRequest from peer %q: %v", peer.Id, err)
//		}
//		if joinClusterResponse.Node == nil {
//			return false, fmt.Errorf("JoingClusterResponse from peer %q did not include peer info", peer.Id, joinClusterResponse)
//		}
//
//		proposal = append(proposal, clusterInfo.buildEtcdNode(peer))
//		if len(proposal) == desiredMemberCount {
//			// We have our cluster
//			break
//		}
//	}
//
//	for _, peer := range proposal {
//		// Note the we send the message to ourselves
//		joinClusterRequest := &JoinClusterRequest{
//			Phase: Phase_PHASE_INITIAL_CLUSTER,
//			ClusterName: m.clusterName,
//			ClusterToken: clusterToken,
//			Nodes: nodes,
//		}
//
//		joinClusterResponse, err := m.rpcJoinCluster(peer.Id, ctx, joinClusterRequest)
//		if err != nil {
//			// TODO: Send a CANCEL message for anything PREPAREd?
//			return false, fmt.Errorf("error from JoinClusterRequest from peer %q: %v", peer.Id, err)
//		}
//		if joinClusterResponse.Node == nil {
//			return false, fmt.Errorf("JoingClusterResponse from peer %q did not include peer info", peer.Id, joinClusterResponse)
//		}
//	}
//
//	return true, nil
//}
//
//type clusterInfo struct {
//	PeerPort int
//	ClientPort int
//	ClusterToken string
//}
//
//func (c *clusterInfo) buildEtcdNode(nodeInfo *privateapi.PeerInfo) (*EtcdNode) {
//	etcdNode := &EtcdNode{
//		// TODO: Include the cluster token (or a portion of it) in the name?
//		Name: buildEtcdNodeName(c.ClusterToken, nodeInfo.Id),
//	}
//	for _, a := range nodeInfo.Addresses {
//		peerUrl := fmt.Sprintf("http://%s:%d", a, c.PeerPort)
//		etcdNode.PeerUrls = append(etcdNode.PeerUrls, peerUrl)
//	}
//	for _, a := range nodeInfo.Addresses {
//		clientUrl := fmt.Sprintf("http://%s:%d", a, c.ClientPort)
//		etcdNode.ClientUrls = append(etcdNode.ClientUrls, clientUrl)
//	}
//
//	return etcdNode
//}
