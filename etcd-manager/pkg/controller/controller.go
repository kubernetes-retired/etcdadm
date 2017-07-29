package controller

import (
	"bytes"
	"context"
	crypto_rand "crypto/rand"
	"encoding/base64"
	"fmt"
	etcd_client "github.com/coreos/etcd/client"
	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"io"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/pkg/etcdclient"
	"kope.io/etcd-manager/pkg/privateapi"
	math_rand "math/rand"
	"sort"
	"sync"
	"time"
)

type EtcdController struct {
	clusterName string

	mutex sync.Mutex

	////model   EtcdCluster
	//baseDir string
	//
	//process *etcdProcess
	//
	//me    privateapi.PeerId
	peers privateapi.Peers

	leadership *leadershipState
}

type leadershipState struct {
	token string
}

type etcdClusterState struct {
	clusterName string
	members     map[string]*etcdclient.EtcdProcessMember
	peers       map[privateapi.PeerId]*etcdClusterPeerInfo
}

func (s *etcdClusterState) String() string {
	var b bytes.Buffer

	fmt.Fprintf(&b, "etcdClusterState\n")
	fmt.Fprintf(&b, "  members:\n")
	for _, m := range s.members {
		fmt.Fprintf(&b, "    %s\n", m)
	}
	fmt.Fprintf(&b, "  peers:\n")
	for _, m := range s.peers {
		fmt.Fprintf(&b, "    %s\n", m)
	}
	return b.String()
}

type etcdClusterPeerInfo struct {
	peer *peer
	info *protoetcd.GetInfoResponse
}

func (p *etcdClusterPeerInfo) String() string {
	return fmt.Sprintf("etcdClusterPeerInfo{peer=%s}", p.peer)
}

type etcdMember struct {
	node *protoetcd.EtcdNode
	peer *privateapi.PeerInfo
}

type peer struct {
	Id    privateapi.PeerId
	info  *privateapi.PeerInfo
	peers privateapi.Peers
}

func (p *peer) String() string {
	s := fmt.Sprintf("peer{%s}", p.info)
	return s
}

func NewEtcdController(clusterName string, peers privateapi.Peers) (*EtcdController, error) {
	//s.mutex.Lock()
	//defer s.mutex.Unlock()

	//if model.DesiredClusterSize < 1 {
	//	return nil, fmt.Errorf("DesiredClusterSize must be > 1")
	//}
	if clusterName == "" {
		return nil, fmt.Errorf("ClusterName is required")
	}
	//if s.etcdClusters[model.ClusterName] != nil {
	//	return nil, fmt.Errorf("cluster %q already registered", model.ClusterName)
	//}
	m := &EtcdController{
		clusterName: clusterName,
		peers:       peers,
		//model:   *model,
		//baseDir: baseDir,
		//peers:   s.peerServer,
		//me:      s.peerServer.MyPeerId(),
	}
	//s.etcdClusters[model.ClusterName] = m
	return m, nil
}

func (m *EtcdController) Run() {
	for {
		ctx := context.Background()
		progress, err := m.run(ctx)
		if err != nil {
			glog.Warningf("unexpected error running etcd cluster reconciliation loop: %v", err)
		}
		if !progress {
			time.Sleep(10 * time.Second)
		}
	}
}

func buildEtcdNodeName(clusterToken string, peerId string) string {
	return clusterToken + "--" + peerId
}

func (m *EtcdController) newPeer(info *privateapi.PeerInfo) *peer {
	p := &peer{
		Id:    privateapi.PeerId(info.Id),
		info:  info,
		peers: m.peers,
	}
	return p
}

func (m *EtcdController) run(ctx context.Context) (bool, error) {
	//m.mutex.Lock()
	//defer m.mutex.Unlock()
	//
	//desiredMemberCount := int(m.model.DesiredClusterSize)
	//

	// Get all (responsive) peers in the discovery cluster
	var peers []*peer
	for _, p := range m.peers.Peers() {
		peers = append(peers, m.newPeer(p))
	}
	sort.SliceStable(peers, func(i, j int) bool {
		return peers[i].Id < peers[j].Id
	})
	glog.Infof("peers: %s", peers)

	// Find self
	var me *peer
	for _, peer := range peers {
		if peer.Id == m.peers.MyPeerId() {
			me = peer
		}
	}
	if me == nil {
		return false, fmt.Errorf("cannot find self %q in list of peers %s", m.peers.MyPeerId(), peers)
	}

	// We only act as controller if we are the leader
	if peers[0].Id != me.Id {
		glog.Infof("we are not leader")
		return false, nil
	}

	if m.leadership == nil {
		leadershipToken, err := m.peers.BecomeLeader(ctx)
		if err != nil {
			return false, fmt.Errorf("error during LeaderNotification: %v", err)
		}

		m.leadership = &leadershipState{
			token: leadershipToken,
		}

		// Wait one cycle after a new leader election
		return true, nil
	}

	clusterState, err := m.updateClusterState(ctx, peers)
	if err != nil {
		return false, fmt.Errorf("error building cluster state: %v", err)
	}

	glog.Infof("etcd cluster state: %s", clusterState)

	glog.V(2).Infof("etcd cluster members: %s", clusterState.members)

	clusterSpec, err := m.loadClusterSpec(ctx, clusterState)
	if err != nil {
		return false, fmt.Errorf("error fetching cluster spec: %v", err)
	}

	//if len(clusterState.Running)  == 0 {
	//	glog.Warningf("No running etcd pods")
	//	// TODO: Recover from backup; if no backup seed backup
	//}

	// Number of peers that are configured as part of this cluster
	configuredMembers := 0

	var clusterConfiguration *protoetcd.EtcdCluster

	for _, peer := range clusterState.peers {
		if peer.info == nil {
			continue
		}
		if peer.info.ClusterConfiguration != nil {
			// TODO: Cross-check that the configuration is the same
			clusterConfiguration = peer.info.ClusterConfiguration

			// TODO: Cross-check that token is the same
			configuredMembers++
		}
	}

	if clusterConfiguration == nil {
		return m.stepStartCluster(ctx, clusterSpec, clusterState)
	}

	if configuredMembers < int(clusterSpec.MemberCount) {
		return m.addNodeToCluster(ctx, clusterState)
	}

	if len(clusterState.members) == 0 {
		glog.Warningf("no members are actually running")
		return false, nil
	}

	glog.Warningf("No additional controller logic")
	return false, nil
	//if m.process == nil {
	//	return m.stepStartCluster(me, peers)
	//}
	//
	//if m.process == nil {
	//	return false, fmt.Errorf("unexpected state in control loop - no etcd process")
	//}
	//
	//members, err := m.process.listMembers()
	//if err != nil {
	//	return false, fmt.Errorf("error querying members from etcd: %v", err)
	//}
	//
	//// TODO: Only if we are leader
	//
	//actualMemberCount := len(members)
	//if actualMemberCount < desiredMemberCount {
	//	glog.Infof("will try to add %d members; existing members: %s", desiredMemberCount - actualMemberCount, members)
	//
	//	// TODO: Randomize peer selection?
	//
	//	membersByName := make(map[string]*etcdProcessMember)
	//	for _, member := range members {
	//		membersByName[member.Name] = member
	//	}
	//
	//	for _, peer := range peers {
	//		peerMemberName := buildEtcdNodeName(m.process.Cluster.ClusterToken, peer.Id)
	//		if membersByName[peerMemberName] != nil {
	//			continue
	//		}
	//
	//		cluster := &EtcdCluster{
	//			ClusterToken: m.process.Cluster.ClusterToken,
	//			ClusterName:  m.process.Cluster.ClusterName,
	//		}
	//		for _, member := range members {
	//			etcdNode := &EtcdNode{
	//				Name:       member.Name,
	//				PeerUrls:   member.PeerURLs,
	//				ClientUrls: member.ClientURLs,
	//			}
	//			cluster.Nodes = append(cluster.Nodes, etcdNode)
	//		}
	//		joinClusterRequest := &JoinClusterRequest{
	//			Cluster: cluster,
	//		}
	//		peerGrpcClient, err := m.peers.GetPeerClient(privateapi.PeerId(peer.Id))
	//		if err != nil {
	//			return false, fmt.Errorf("error getting peer client %q: %v", peer.Id, err)
	//		}
	//		peerClient := NewEtcdManagerServiceClient(peerGrpcClient)
	//		joinClusterResponse, err := peerClient.JoinCluster(ctx, joinClusterRequest)
	//		if err != nil {
	//			return false, fmt.Errorf("error from JoinClusterRequest from peer %q: %v", peer.Id, err)
	//		}
	//		if joinClusterResponse.Node == nil {
	//			return false, fmt.Errorf("JoingClusterResponse from peer %q did not include peer info", peer.Id, joinClusterResponse)
	//		}
	//		peerURLs := joinClusterResponse.Node.PeerUrls
	//		//var peerURLs []string
	//		//for _, address := range peer.Addresses {
	//		//	peerURL := fmt.Sprintf("http://%s:%d", address, m.model.PeerPort)
	//		//	peerURLs = append(peerURLs, peerURL)
	//		//}
	//		glog.Infof("will try to add member %s @ %s", peerMemberName, peerURLs)
	//		member, err := m.process.addMember(peerMemberName, peerURLs)
	//		if err != nil {
	//			return false, fmt.Errorf("failed to add peer %s %s: %v", peerMemberName, peerURLs, err)
	//		}
	//		glog.Infof("Added member: %s", member)
	//		actualMemberCount++
	//		if actualMemberCount == desiredMemberCount {
	//			break
	//		}
	//	}
	//}
	//
	//// TODO: Eventually replace unhealthy members?
	//
	//return false, nil
}

func (m *EtcdController) loadClusterSpec(ctx context.Context, etcdClusterState *etcdClusterState) (*protoetcd.ClusterSpec, error) {
	clusterSpec := &protoetcd.ClusterSpec{}

	key := "/kope.io/etcd-manager/" + m.clusterName + "/spec"
	b, err := etcdClusterState.etcdGet(ctx, key)
	if err == nil {
		err := proto.Unmarshal(b, clusterSpec)
		if err != nil {
			return nil, fmt.Errorf("error parsing cluster spec from etcd %q: %v", key, err)
		}
		return clusterSpec, nil
	} else {
		glog.Warningf("unable to read cluster spec from etcd: %v", err)
	}

	// TODO: Source from etcd / from state / from backup
	glog.Warningf("Using hard coded ClusterSpec")
	clusterSpec = &protoetcd.ClusterSpec{
		MemberCount: 3,
	}
	return clusterSpec, nil

}
func randomToken() string {
	b := make([]byte, 16, 16)
	_, err := io.ReadFull(crypto_rand.Reader, b)
	if err != nil {
		glog.Fatalf("error generating random token: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func (m *EtcdController) updateClusterState(ctx context.Context, peers []*peer) (*etcdClusterState, error) {
	clusterState := &etcdClusterState{
		peers: make(map[privateapi.PeerId]*etcdClusterPeerInfo),
	}

	// Collect info from each peer
	for _, peer := range peers {
		getInfoRequest := &protoetcd.GetInfoRequest{}

		getInfoResponse, err := peer.rpcGetInfo(ctx, getInfoRequest)
		if err != nil {
			// peers should only be healthy peers, so we don't expect an error
			return nil, fmt.Errorf("error from GetInfo from peer %q: %v", peer.Id, err)
		}

		clusterState.peers[peer.Id] = &etcdClusterPeerInfo{
			info: getInfoResponse,
			peer: peer,
		}
	}

	for _, p := range clusterState.peers {
		// TODO: Filter by peer state?
		if p.info.NodeConfiguration == nil || len(p.info.NodeConfiguration.ClientUrls) == 0 {
			continue
		}

		cfg := etcd_client.Config{
			Endpoints: p.info.NodeConfiguration.ClientUrls,
			Transport: etcd_client.DefaultTransport,
			// set timeout per request to fail fast when the target endpoint is unavailable
			HeaderTimeoutPerRequest: time.Second,
		}
		etcdClient, err := etcd_client.New(cfg)
		if err != nil {
			glog.Warningf("unable to reach member %s: %v", p, err)
			continue
		}

		membersAPI := etcd_client.NewMembersAPI(etcdClient)
		members, err := membersAPI.List(ctx)
		if err != nil {
			glog.Warningf("unable to reach member %s: %v", p, err)
			continue
		}

		clusterState.members = make(map[string]*etcdclient.EtcdProcessMember)
		for _, m := range members {
			clusterState.members[m.Name] = &etcdclient.EtcdProcessMember{
				Name:       m.Name,
				Id:         m.ID,
				PeerURLs:   m.PeerURLs,
				ClientURLs: m.ClientURLs,
			}
		}
		break

		//clientURL := p.info.NodeConfiguration.ClientUrls[0]
		//etcdClient := etcdclient.NewClient(clientURL)
		//members, err := etcdClient.ListMembers(ctx)
		//if err != nil {
		//	glog.Warningf("unable to reach member %s: %v", p, err)
		//}
		//if err == nil {
		//	clusterState.members = make(map[string]*etcdclient.EtcdProcessMember)
		//	for _, m := range members {
		//		clusterState.members[m.Name] = m
		//	}
		//	break
		//}
	}

	// TODO: Query each cluster member to see if it is healthy

	// TODO: Query each cluster to try to find the members?  the leaders ?

	return clusterState, nil
}

func (s *etcdClusterState) etcdAddMember(ctx context.Context, nodeInfo *protoetcd.EtcdNode) (*etcdclient.EtcdProcessMember, error) {
	for _, member := range s.members {
		if len(member.ClientURLs) == 0 {
			glog.Warningf("skipping member with no ClientURLs: %v", member)
			continue
		}
		clientURL := member.ClientURLs[0]
		etcdClient := etcdclient.NewClient(clientURL)
		member, err := etcdClient.AddMember(ctx, nodeInfo.Name, nodeInfo.PeerUrls)
		if err != nil {
			return nil, fmt.Errorf("error trying to add member: %v", err)
		}
		return member, nil
	}
	return nil, fmt.Errorf("unable to reach any cluster member, when trying to add new member")
}

func (s *etcdClusterState) etcdGet(ctx context.Context, key string) ([]byte, error) {
	for _, member := range s.members {
		if len(member.ClientURLs) == 0 {
			glog.Warningf("skipping member with no ClientURLs: %v", member)
			continue
		}

		cfg := etcd_client.Config{
			Endpoints: member.ClientURLs,
			Transport: etcd_client.DefaultTransport,
			// set timeout per request to fail fast when the target endpoint is unavailable
			HeaderTimeoutPerRequest: time.Second,
		}
		etcdClient, err := etcd_client.New(cfg)
		if err != nil {
			glog.Warningf("unable to reach member %s: %v", member, err)
			continue
		}

		keysAPI := etcd_client.NewKeysAPI(etcdClient)
		// TODO: Quorum?  Read from all nodes?
		opts := &etcd_client.GetOptions{
			Quorum: true,
		}
		response, err := keysAPI.Get(ctx, key, opts)
		if err != nil {
			return nil, fmt.Errorf("error reading from member %s: %v", member, err)
		}

		if response.Node == nil {
			return nil, fmt.Errorf("node not set reading from member %s", member)
		}

		val, err := base64.RawStdEncoding.DecodeString(response.Node.Value)
		if err != nil {
			return nil, fmt.Errorf("error decoding key %q: %v", key, err)
		}

		return val, nil
	}

	return nil, fmt.Errorf("unable to reach any cluster member, when trying to read key %q", key)
}

//func (m *EtcdController) processCluster(cluster *etcdClusterState) {
//	// If there is no member at all, we can try to start the cluster
//	if len(cluster.members) == 0 {
//		return m.stepStartCluster()
//	}
//
//	var expectedClusterState *EtcdCluster
//	var err error
//	leader := cluster.Leader()
//	if leader != nil {
//		expectedClusterState, err = etcdQueryState(leader, expectedClusterState)
//		if err != nil {
//			// TODO: Not found - maybe fall back to bootstrap expectation?  Or better.. to backup manager...
//			return fmt.Errorf("error querying leader for expected query state: %v", err)
//		}
//	}
//	if expectedClusterState == nil {
//		for _, member := range cluster.members {
//			if member.Healthy {
//				expectedClusterState, err = etcdQueryState(member, expectedClusterState)
//				if err != nil {
//					return fmt.Errorf("error querying member for expected query state: %v", err)
//				}
//				if expectedClusterState != nil {
//					break
//				}
//			}
//		}
//	}
//	if expectedClusterState == nil {
//		// TODO: This is the no healthy nodes case
//		return fmt.Errorf("unable to find expected query state: %v", err)
//	}
//
//	if expectedClusterState.MemberCount ==
//}

func (m *EtcdController) addNodeToCluster(ctx context.Context, clusterState *etcdClusterState) (bool, error) {
	var peersMissingFromEtcd []*etcdClusterPeerInfo
	var idlePeers []*etcdClusterPeerInfo
	for _, peer := range clusterState.peers {
		if peer.info != nil {
			etcdMember := clusterState.members[peer.info.NodeConfiguration.Name]
			if etcdMember == nil {
				peersMissingFromEtcd = append(peersMissingFromEtcd, peer)
			}
		} else {
			idlePeers = append(idlePeers, peer)
		}
	}

	if len(peersMissingFromEtcd) != 0 {
		glog.Infof("detected etcd servers not added to etcd cluster: %v", peersMissingFromEtcd)

		peer := peersMissingFromEtcd[math_rand.Intn(len(peersMissingFromEtcd))]
		glog.Infof("trying to add peer: %v", peer)

		member, err := clusterState.etcdAddMember(ctx, peer.info.NodeConfiguration)
		if err != nil {
			return false, fmt.Errorf("error adding peer %q to cluster: %v", peer, err)
		}
		glog.Infof("Adding existing member to cluster: %s", member)
		// We made some progress here; give it a cycle to join & sync
		return true, nil
	}

	// We need to start etcd on a new node
	if len(idlePeers) != 0 {
		peer := idlePeers[math_rand.Intn(len(idlePeers))]
		glog.Infof("will try to start new peer: %v", peer)

		var nodes []*protoetcd.EtcdNode
		for _, member := range clusterState.members {
			node := &protoetcd.EtcdNode{
				Name:       member.Name,
				ClientUrls: member.ClientURLs,
				PeerUrls:   member.PeerURLs,
			}
			nodes = append(nodes, node)
		}

		clusterToken := ""
		for _, peer := range clusterState.peers {
			if peer.info != nil && peer.info.ClusterConfiguration != nil {
				clusterToken = peer.info.ClusterConfiguration.ClusterToken
			}
		}
		if clusterToken == "" {
			// Should be unreachable
			return false, fmt.Errorf("unable to determine cluster token")
		}

		{
			joinClusterRequest := &protoetcd.JoinClusterRequest{
				LeadershipToken: m.leadership.token,
				Phase:           protoetcd.Phase_PHASE_PREPARE,
				ClusterName:     clusterState.clusterName,
				ClusterToken:    clusterToken,
				Nodes:           nodes,
			}

			joinClusterResponse, err := peer.peer.rpcJoinCluster(ctx, joinClusterRequest)
			if err != nil {
				return false, fmt.Errorf("error from JoinClusterRequest from peer %q: %v", peer.peer.Id, err)
			}
			glog.V(2).Infof("JoinCluster returned %s", joinClusterResponse)
		}

		{
			joinClusterRequest := &protoetcd.JoinClusterRequest{
				LeadershipToken: m.leadership.token,
				Phase:           protoetcd.Phase_PHASE_JOIN_EXISTING,
				ClusterName:     clusterState.clusterName,
				ClusterToken:    clusterToken,
				Nodes:           nodes,
			}

			joinClusterResponse, err := peer.peer.rpcJoinCluster(ctx, joinClusterRequest)
			if err != nil {
				return false, fmt.Errorf("error from JoinClusterRequest from peer %q: %v", peer.peer.Id, err)
			}
			glog.V(2).Infof("JoinCluster returned %s", joinClusterResponse)
		}

		addedMember, err := clusterState.etcdAddMember(ctx, peer.info.NodeConfiguration)
		if err != nil {
			return false, fmt.Errorf("error adding peer %q to cluster: %v", peer, err)
		}
		glog.Infof("Added new member to cluster: %s", addedMember)
		// We made some progress here; give it a cycle to join & sync
		return true, nil
	}

	glog.Infof("Want to expand cluster but no available nodes")
	return false, nil
}

func (p *peer) rpcJoinCluster(ctx context.Context, joinClusterRequest *protoetcd.JoinClusterRequest) (*protoetcd.JoinClusterResponse, error) {
	peerGrpcClient, err := p.peers.GetPeerClient(p.Id)
	if err != nil {
		return nil, fmt.Errorf("error getting peer client %q: %v", p.Id, err)
	}
	peerClient := protoetcd.NewEtcdManagerServiceClient(peerGrpcClient)
	return peerClient.JoinCluster(ctx, joinClusterRequest)
}

func (p *peer) rpcGetInfo(ctx context.Context, request *protoetcd.GetInfoRequest) (*protoetcd.GetInfoResponse, error) {
	peerGrpcClient, err := p.peers.GetPeerClient(p.Id)
	if err != nil {
		return nil, fmt.Errorf("error getting peer client %q: %v", p.Id, err)
	}
	peerClient := protoetcd.NewEtcdManagerServiceClient(peerGrpcClient)
	return peerClient.GetInfo(ctx, request)
}

func (m *EtcdController) stepStartCluster(ctx context.Context, clusterSpec *protoetcd.ClusterSpec, clusterState *etcdClusterState) (bool, error) {
	desiredMemberCount := int(clusterSpec.MemberCount)
	quorumSize := (desiredMemberCount / 2) + 1

	if len(clusterState.peers) < quorumSize {
		glog.Infof("Insufficient peers to form a quorum %d, won't proceed", quorumSize)
		return false, nil
	}

	if len(clusterState.peers) < desiredMemberCount {
		// TODO: We should relax this, but that requires etcd to support an explicit quorum setting, or we can create dummy entries
		glog.Infof("Insufficient peers to form full cluster %d, won't proceed", quorumSize)
		return false, nil
	}

	clusterToken := randomToken()

	var proposal []*etcdClusterPeerInfo
	for _, peer := range clusterState.peers {
		proposal = append(proposal, peer)
		if len(proposal) == desiredMemberCount {
			// We have our cluster
			break
		}
	}

	if len(proposal) < desiredMemberCount {
		glog.Fatalf("Need to add dummy peers to force quorum size :-(")
	}

	var proposedNodes []*protoetcd.EtcdNode
	for _, p := range proposal {
		proposedNodes = append(proposedNodes, p.info.NodeConfiguration)
	}

	for _, p := range proposal {
		// Note the we send the message to ourselves
		joinClusterRequest := &protoetcd.JoinClusterRequest{
			LeadershipToken: m.leadership.token,

			Phase:        protoetcd.Phase_PHASE_PREPARE,
			ClusterName:  m.clusterName,
			ClusterToken: clusterToken,
			Nodes:        proposedNodes,
		}

		joinClusterResponse, err := p.peer.rpcJoinCluster(ctx, joinClusterRequest)
		if err != nil {
			// TODO: Send a CANCEL message for anything PREPAREd?
			return false, fmt.Errorf("error from JoinClusterRequest from peer %q: %v", p.peer, err)
		}
		glog.V(2).Infof("JoinClusterResponse: %s", joinClusterResponse)
		//if joinClusterResponse.Node == nil {
		//	return false, fmt.Errorf("JoingClusterResponse from peer %q did not include peer info", p.peer, joinClusterResponse)
		//}
	}

	for _, p := range proposal {
		// Note the we send the message to ourselves
		joinClusterRequest := &protoetcd.JoinClusterRequest{
			LeadershipToken: m.leadership.token,

			Phase:        protoetcd.Phase_PHASE_INITIAL_CLUSTER,
			ClusterName:  m.clusterName,
			ClusterToken: clusterToken,
			Nodes:        proposedNodes,
		}

		joinClusterResponse, err := p.peer.rpcJoinCluster(ctx, joinClusterRequest)
		if err != nil {
			// TODO: Send a CANCEL message for anything PREPAREd?
			return false, fmt.Errorf("error from JoinClusterRequest from peer %q: %v", p.peer, err)
		}
		glog.V(2).Infof("JoinClusterResponse: %s", joinClusterResponse)
		//if joinClusterResponse.Node == nil {
		//	return false, fmt.Errorf("JoingClusterResponse from peer %q did not include peer info", peer.Id, joinClusterResponse)
		//}
	}

	return true, nil
}

//type clusterInfo struct {
//	PeerPort     int
//	ClientPort   int
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
