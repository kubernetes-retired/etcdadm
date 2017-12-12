package controller

import (
	"bytes"
	"context"
	crypto_rand "crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	math_rand "math/rand"
	"sort"
	"sync"
	"time"

	etcd_client "github.com/coreos/etcd/client"
	"github.com/golang/glog"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/pkg/backup"
	"kope.io/etcd-manager/pkg/contextutil"
	"kope.io/etcd-manager/pkg/etcdclient"
	"kope.io/etcd-manager/pkg/locking"
	"kope.io/etcd-manager/pkg/privateapi"
)

const removeUnhealthyDeadline = time.Minute // TODO: increase

// defaultCycleInterval is the default value of EtcdController::CycleInterval
const defaultCycleInterval = 10 * time.Second

type EtcdController struct {
	clusterName string
	backupStore backup.Store

	mutex sync.Mutex

	////model   EtcdCluster
	//baseDir string
	//
	//process *etcdProcess
	//
	//me    privateapi.PeerId
	peers privateapi.Peers

	leaderLock      locking.Lock
	leaderLockGuard locking.LockGuard

	leadership *leadershipState
	peerState  map[privateapi.PeerId]*peerState

	InitialClusterSpecProvider InitialClusterSpecProvider

	// CycleInterval is the time to wait in between iterations of the state synchronization loop, when no progress has been made previously
	CycleInterval time.Duration
}

// peerState holds persistent information about a peer
type peerState struct {
	// last time etcd member responded to us
	lastEtcdHealthy time.Time
}

type leadershipState struct {
	token string
	acked map[privateapi.PeerId]bool
}

type EtcdMemberId string

type etcdClusterState struct {
	//clusterName    string
	members        map[EtcdMemberId]*etcdclient.EtcdProcessMember
	peers          map[privateapi.PeerId]*etcdClusterPeerInfo
	healthyMembers map[EtcdMemberId]*etcdclient.EtcdProcessMember
}

func (s *etcdClusterState) String() string {
	var b bytes.Buffer

	fmt.Fprintf(&b, "etcdClusterState\n")
	fmt.Fprintf(&b, "  members:\n")
	for id, m := range s.members {
		fmt.Fprintf(&b, "    %s\n", m)
		if s.healthyMembers[id] == nil {
			fmt.Fprintf(&b, "      NOT HEALTHY\n")
		}
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

func NewEtcdController(leaderLock locking.Lock, backupStore backup.Store, clusterName string, peers privateapi.Peers, initialClusterState InitialClusterSpecProvider) (*EtcdController, error) {
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
		backupStore: backupStore,
		peers:       peers,
		leaderLock:  leaderLock,
		//model:   *model,
		//baseDir: baseDir,
		//peers:   s.peerServer,
		//me:      s.peerServer.MyPeerId(),
		InitialClusterSpecProvider: initialClusterState,
		CycleInterval:              defaultCycleInterval,
	}
	//s.etcdClusters[model.ClusterName] = m
	return m, nil
}

func (m *EtcdController) Run(ctx context.Context) {
	contextutil.Forever(ctx,
		time.Millisecond, // We do our own sleeping
		func() {
			progress, err := m.run(ctx)
			if err != nil {
				glog.Warningf("unexpected error running etcd cluster reconciliation loop: %v", err)
			}
			if !progress {
				contextutil.Sleep(ctx, m.CycleInterval)
			}
		})
}

//func buildEtcdNodeName(clusterToken string, peerId string) string {
//	return clusterToken + "--" + peerId
//}

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

	glog.Infof("starting controller iteration")

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

	// We only try to act as controller if we are the leader (lowest id)
	if peers[0].Id != me.Id {
		glog.V(4).Infof("we are not leader")

		if m.leaderLockGuard != nil {
			glog.Infof("releasing leader lock")
			if err := m.leaderLockGuard.Release(); err != nil {
				return false, fmt.Errorf("failed to release leader lock guard: %v", err)
			}
			m.leaderLockGuard = nil
		}

		return false, nil
	}

	// We now try to obtain the leader-lock; this is how we don't form multiple clusters if we split-brain,
	// even if there are enough nodes to form 2 quorums
	if m.leaderLock != nil && m.leaderLockGuard == nil {
		leaderLockGuard, err := m.leaderLock.Acquire(ctx, string(me.Id))
		if err != nil {
			return false, fmt.Errorf("error acquiring leader lock: %v", err)
		}
		if leaderLockGuard == nil {
			glog.Infof("could not acquire leader lock")
			return false, nil
		}
		m.leaderLockGuard = leaderLockGuard
	}

	// If we believe we are the leader, we try to tell everyone we know
	if m.leadership == nil {
		acked, leadershipToken, err := m.peers.BecomeLeader(ctx)
		if err != nil {
			return false, fmt.Errorf("error during LeaderNotification: %v", err)
		}

		ackedMap := make(map[privateapi.PeerId]bool)
		for _, peer := range acked {
			ackedMap[peer] = true
		}
		m.leadership = &leadershipState{
			token: leadershipToken,
			acked: ackedMap,
		}

		// reset our peer state after a leadership transition
		// TODO: How do we lose leadership
		m.peerState = make(map[privateapi.PeerId]*peerState)

		// Wait one cycle after a new leader election
		return true, nil
	}

	// Check that all peers have acked the leader
	{
		for _, peer := range peers {
			if !m.leadership.acked[peer.Id] {
				glog.Infof("peer %q has not acked our leadership; resigning leadership", peer)
				m.leadership = nil

				// Wait one cycle after leadership changes
				return true, nil
			}
		}
	}

	// Query all our peers to try to find the actual state of etcd on each node
	clusterState, err := m.updateClusterState(ctx, peers)
	if err != nil {
		return false, fmt.Errorf("error building cluster state: %v", err)
	}
	glog.Infof("etcd cluster state: %s", clusterState)
	glog.V(2).Infof("etcd cluster members: %s", clusterState.members)

	now := time.Now()

	for id := range clusterState.members {
		ps := m.peerState[privateapi.PeerId(id)]
		if ps == nil {
			ps = &peerState{
				lastEtcdHealthy: now, // We mark it as healthy, so we always wait before removing it
			}
			m.peerState[privateapi.PeerId(id)] = ps
		}
		if clusterState.healthyMembers[id] != nil {
			ps.lastEtcdHealthy = now
		}
	}

	// Number of peers that are configured as part of this cluster
	configuredMembers := 0

	for _, peer := range clusterState.peers {
		if peer.info == nil {
			continue
		}
		if peer.info.EtcdConfigured {
			//// TODO: Cross-check that the configuration is the same
			//clusterConfiguration = peer.info.ClusterConfiguration

			// TODO: Cross-check that token is the same
			configuredMembers++
		}
	}

	// Determine what our desired state is
	clusterSpec, err := m.loadClusterSpec(ctx, clusterState, configuredMembers != 0)
	if err != nil {
		return false, fmt.Errorf("error fetching cluster spec: %v", err)
	}
	glog.Infof("spec %v", clusterSpec)

	//if len(clusterState.Running)  == 0 {
	//	glog.Warningf("No running etcd pods")
	//	// TODO: Recover from backup; if no backup seed backup
	//}

	if configuredMembers == 0 {
		return m.stepStartCluster(ctx, clusterSpec, clusterState)
	}

	if len(clusterState.members) < int(clusterSpec.MemberCount) {
		// TODO: Still backup when we have an under-quorum cluster
		glog.Infof("etcd has %d members registered, we want %d; will try to expand cluster", len(clusterState.members), clusterSpec.MemberCount)
		return m.addNodeToCluster(ctx, clusterState)
	}

	if len(clusterState.members) == 0 {
		glog.Warningf("no members are actually running")
		return false, nil
	}

	if configuredMembers > int(clusterSpec.MemberCount) {
		// TODO: Still backup before mutating the cluster
		return m.removeNodeFromCluster(ctx, clusterSpec, clusterState, true)
	}

	// healthy members
	if len(clusterState.healthyMembers) < int(len(clusterState.members)) {
		glog.Infof("etcd has unhealthy members")
		// TODO: Wait longer in case of a flake
		// TODO: Still backup before mutating the cluster
		return m.removeNodeFromCluster(ctx, clusterSpec, clusterState, false)
	}

	backup, err := m.doClusterBackup(ctx, clusterSpec, clusterState)
	if err != nil {
		glog.Warningf("error during backup: %v", err)
	} else {
		glog.Infof("took backup: %v", backup)
	}

	glog.Infof("controller loop complete")

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

func (m *EtcdController) loadClusterSpec(ctx context.Context, etcdClusterState *etcdClusterState, etcdIsRunning bool) (*protoetcd.ClusterSpec, error) {
	key := "/kope.io/etcd-manager/" + m.clusterName + "/spec"
	b, err := etcdClusterState.etcdGet(ctx, key)
	if err == nil {
		state := &protoetcd.ClusterSpec{}
		err := protoetcd.FromJson(b, state)
		if err != nil {
			return nil, fmt.Errorf("error parsing cluster spec from etcd %q: %v", key, err)
		}
		return state, nil
	} else {
		glog.Warningf("unable to read cluster spec from etcd: %v", err)
	}

	{
		var state *protoetcd.ClusterSpec

		backups, err := m.backupStore.ListBackups()
		if err != nil {
			return nil, fmt.Errorf("error listing backups: %v", err)
		}

		for i := len(backups) - 1; i >= 0; i-- {
			backup := backups[i]

			state, err = m.backupStore.LoadClusterState(backup)
			if err != nil {
				glog.Warningf("error reading cluster state in %q: %v", backup, err)
				continue
			}
			if state == nil {
				glog.Warningf("cluster state not found in %q", backup)
				continue
			} else {
				break
			}
		}

		if state != nil {
			data, err := protoetcd.ToJson(state)
			if err != nil {
				return nil, fmt.Errorf("error serializing cluster spec: %v", err)
			}

			if etcdIsRunning {
				err = etcdClusterState.etcdCreate(ctx, key, data)
				if err != nil {
					// Concurrent leader wrote this?
					return nil, fmt.Errorf("error writing cluster spec back to etcd: %v", err)
				}
			}

			return state, nil
		}
	}

	// TODO: Source from etcd / from state / from backup
	if m.InitialClusterSpecProvider == nil {
		return nil, fmt.Errorf("no cluster spec found, and no InitialClusterSpecProvider provider")
	}
	state, err := m.InitialClusterSpecProvider.GetInitialClusterSpec()
	if err != nil {
		return nil, fmt.Errorf("no cluster spec found, and error from InitialClusterSpecProvider provider: %v", err)
	}
	return state, nil

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

		clusterState.members = make(map[EtcdMemberId]*etcdclient.EtcdProcessMember)
		clusterState.healthyMembers = make(map[EtcdMemberId]*etcdclient.EtcdProcessMember)
		for _, m := range members {
			// Note that members don't necessarily have names, when they are added but not yet merged
			memberId := EtcdMemberId(m.ID)
			if memberId == "" {
				glog.Fatalf("etcd member did not have ID: %v", m)
			}
			clusterState.members[memberId] = &etcdclient.EtcdProcessMember{
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

	// Query each cluster member to see if it is healthy
	for id, member := range clusterState.members {
		etcdClient, err := member.Client()
		if err != nil {
			glog.Warningf("health-check unable to reach member %s: %v", id, err)
			continue
		}

		membersAPI := etcd_client.NewMembersAPI(etcdClient)
		_, err = membersAPI.List(ctx)
		if err != nil {
			glog.Warningf("health-check unable to reach member %s: %v", id, err)
			continue
		}

		// TODO: Cross-check members?

		clusterState.healthyMembers[id] = member
	}

	// TODO: Query each cluster to try to find the members?  the leaders ?

	return clusterState, nil
}

func (s *etcdClusterState) etcdAddMember(ctx context.Context, nodeInfo *protoetcd.EtcdNode) (*etcdclient.EtcdProcessMember, error) {
	// TODO: Try all peer urls?
	peerURL := nodeInfo.PeerUrls[0]

	for _, member := range s.members {
		etcdClient, err := member.Client()
		if err != nil {
			glog.Warningf("unable to build client for member %s: %v", member.Name, err)
			continue
		}

		membersAPI := etcd_client.NewMembersAPI(etcdClient)
		_, err = membersAPI.Add(ctx, peerURL)
		if err != nil {
			glog.Warningf("unable to add member %s on peer %s: %v", peerURL, member.Name, err)
			continue
		}

		return member, nil
	}
	return nil, fmt.Errorf("unable to reach any cluster member, when trying to add new member %q", peerURL)
}

func (s *etcdClusterState) etcdRemoveMember(ctx context.Context, memberID string) error {
	for id, member := range s.members {
		etcdClient, err := member.Client()
		if err != nil {
			glog.Warningf("unable to build client for member %s: %v", member.Name, err)
			continue
		}

		membersAPI := etcd_client.NewMembersAPI(etcdClient)
		err = membersAPI.Remove(ctx, memberID)
		if err != nil {
			glog.Warningf("Remove member call failed on %s: %v", id, err)
			continue
		}
		return nil
	}
	return fmt.Errorf("unable to reach any cluster member, when trying to remove member %q", memberID)
}

func (s *etcdClusterState) etcdGet(ctx context.Context, key string) (string, error) {
	for _, member := range s.members {
		if len(member.ClientURLs) == 0 {
			glog.Warningf("skipping member with no ClientURLs: %v", member)
			continue
		}

		etcdClient, err := member.Client()
		if err != nil {
			glog.Warningf("unable to build client for member %s: %v", member.Name, err)
			continue
		}

		keysAPI := etcd_client.NewKeysAPI(etcdClient)
		// TODO: Quorum?  Read from all nodes?
		opts := &etcd_client.GetOptions{
			Quorum: true,
		}
		response, err := keysAPI.Get(ctx, key, opts)
		if err != nil {
			if clusterError, ok := err.(*etcd_client.ClusterError); ok {
				glog.Warningf("error reading from member %s: %v", member, clusterError)
				continue
			} else {
				return "", fmt.Errorf("error reading from member %s: %v", member, err)
			}
		}

		if response.Node == nil {
			return "", fmt.Errorf("node not set reading from member %s", member)
		}

		return response.Node.Value, nil
	}

	return "", fmt.Errorf("unable to reach any cluster member, when trying to read key %q", key)
}

func (s *etcdClusterState) etcdCreate(ctx context.Context, key string, value string) error {
	for _, member := range s.members {
		if len(member.ClientURLs) == 0 {
			glog.Warningf("skipping member with no ClientURLs: %v", member)
			continue
		}

		etcdClient, err := member.Client()
		if err != nil {
			glog.Warningf("unable to build client for member %s: %v", member.Name, err)
			continue
		}

		keysAPI := etcd_client.NewKeysAPI(etcdClient)

		options := &etcd_client.SetOptions{
			PrevExist: etcd_client.PrevNoExist,
		}
		response, err := keysAPI.Set(ctx, key, value, options)
		if err != nil {
			return fmt.Errorf("error creating %q on member %s: %v", key, member, err)
		}
		glog.V(2).Infof("Set returned %v", response)

		return nil
	}

	return fmt.Errorf("unable to reach any cluster member, when trying to write key %q", key)
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
		if peer.info != nil && peer.info.EtcdConfigured {
			var etcdMember *etcdclient.EtcdProcessMember
			for _, member := range clusterState.members {
				if member.Name == peer.info.NodeConfiguration.Name {
					etcdMember = member
				}
			}
			if etcdMember == nil {
				peersMissingFromEtcd = append(peersMissingFromEtcd, peer)
			}
		} else {
			idlePeers = append(idlePeers, peer)
		}
	}

	//if len(peersMissingFromEtcd) != 0 {
	//	glog.Infof("detected etcd servers not added to etcd cluster: %v", peersMissingFromEtcd)
	//
	//	peer := peersMissingFromEtcd[math_rand.Intn(len(peersMissingFromEtcd))]
	//	glog.Infof("trying to add peer: %v", peer)
	//
	//	member, err := clusterState.etcdAddMember(ctx, peer.info.NodeConfiguration)
	//	if err != nil {
	//		return false, fmt.Errorf("error adding peer %q to cluster: %v", peer, err)
	//	}
	//	glog.Infof("Adding existing member to cluster: %s", member)
	//	// We made some progress here; give it a cycle to join & sync
	//	return true, nil
	//}

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

		nodes = append(nodes, peer.info.NodeConfiguration)

		clusterToken := ""
		for _, peer := range clusterState.peers {
			if peer.info != nil && peer.info.EtcdConfigured && peer.info.ClusterToken != "" {
				clusterToken = peer.info.ClusterToken
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
				ClusterName:     m.clusterName,
				ClusterToken:    clusterToken,
				Nodes:           nodes,
			}

			joinClusterResponse, err := peer.peer.rpcJoinCluster(ctx, joinClusterRequest)
			if err != nil {
				return false, fmt.Errorf("error from JoinClusterRequest from peer %q: %v", peer.peer.Id, err)
			}
			glog.V(2).Infof("JoinCluster returned %s", joinClusterResponse)
		}

		// We have to add the peer to etcd before starting it
		// * because the node fails to start if it is not added to the cluster first
		// * and because we want etcd to be our source of truth
		glog.Infof("Adding member to cluster: %s", peer.info.NodeConfiguration)
		_, err := clusterState.etcdAddMember(ctx, peer.info.NodeConfiguration)
		if err != nil {
			return false, fmt.Errorf("error adding peer %q to cluster: %v", peer, err)
		}

		{
			joinClusterRequest := &protoetcd.JoinClusterRequest{
				LeadershipToken: m.leadership.token,
				Phase:           protoetcd.Phase_PHASE_JOIN_EXISTING,
				ClusterName:     m.clusterName,
				ClusterToken:    clusterToken,
				Nodes:           nodes,
			}

			joinClusterResponse, err := peer.peer.rpcJoinCluster(ctx, joinClusterRequest)
			if err != nil {
				return false, fmt.Errorf("error from JoinClusterRequest from peer %q: %v", peer.peer.Id, err)
			}
			glog.V(2).Infof("JoinCluster returned %s", joinClusterResponse)
		}

		// We made some progress here; give it a cycle to join & sync
		return true, nil
	}

	glog.Infof("Want to expand cluster but no available nodes")
	return false, nil
}

func (m *EtcdController) doClusterBackup(ctx context.Context, clusterSpec *protoetcd.ClusterSpec, clusterState *etcdClusterState) (*protoetcd.DoBackupResponse, error) {
	for _, peer := range clusterState.peers {
		doBackupRequest := &protoetcd.DoBackupRequest{
			LeadershipToken: m.leadership.token,
			ClusterName:     m.clusterName,
			Storage:         m.backupStore.Spec(),
			State:           clusterSpec,
		}

		doBackupResponse, err := peer.peer.rpcDoBackup(ctx, doBackupRequest)
		if err != nil {
			glog.Warningf("peer gave error while trying to do backup: %v", err)
		} else {
			glog.V(2).Infof("backup response: %v", doBackupResponse)
			return doBackupResponse, nil
		}
	}

	return nil, fmt.Errorf("no peer was able to perform a backup")
}

func (p *peer) rpcDoBackup(ctx context.Context, doBackupRequest *protoetcd.DoBackupRequest) (*protoetcd.DoBackupResponse, error) {
	peerGrpcClient, err := p.peers.GetPeerClient(p.Id)
	if err != nil {
		return nil, fmt.Errorf("error getting peer client %q: %v", p.Id, err)
	}
	peerClient := protoetcd.NewEtcdManagerServiceClient(peerGrpcClient)
	return peerClient.DoBackup(ctx, doBackupRequest)
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

func (m *EtcdController) removeNodeFromCluster(ctx context.Context, clusterSpec *protoetcd.ClusterSpec, clusterState *etcdClusterState, removeHealthy bool) (bool, error) {
	//desiredMemberCount := int(clusterSpec.MemberCount)
	//quorumSize := (desiredMemberCount / 2) + 1

	// TODO: Sanity checks that we aren't about to break the cluster

	var victim *etcdclient.EtcdProcessMember

	now := time.Now()

	// Favor an unhealthy member
	if len(clusterState.healthyMembers) < len(clusterState.members) {
		for id, member := range clusterState.members {
			if clusterState.healthyMembers[id] == nil {
				if !removeHealthy {
					// TODO: remove most unhealthy member?
					peerState := m.peerState[privateapi.PeerId(id)]
					if peerState == nil {
						glog.Fatalf("peerState unexpectedly nil")
					}
					age := now.Sub(peerState.lastEtcdHealthy)
					if age < removeUnhealthyDeadline {
						glog.Infof("peer %v is unhealthy, but waiting for %s (currently %s)", member, removeUnhealthyDeadline, age)
						continue
					}

				}

				victim = member
				break
			}
		}
	}

	if victim == nil && !removeHealthy {
		glog.Infof("want to remove unhealthy members, but waiting to verify it doesn't recover")
		return false, nil
	}

	if victim == nil {
		// Pick randomly...
		// TODO: Sufficient to rely on map randomization?

		// TODO: Avoid killing the leader

		for _, member := range clusterState.members {
			victim = member
			break
		}
	}

	if victim == nil {
		return false, fmt.Errorf("unable to pick a member to remove")
	}

	glog.Infof("removing node from etcd cluster: %v", victim)

	err := clusterState.etcdRemoveMember(ctx, victim.Id)
	if err != nil {
		return false, fmt.Errorf("failed to remove member %q: %v", victim, err)
	}

	return true, nil
}

func quorumSize(desiredMemberCount int) int {
	return (desiredMemberCount / 2) + 1
}

func (m *EtcdController) stepStartCluster(ctx context.Context, clusterSpec *protoetcd.ClusterSpec, clusterState *etcdClusterState) (bool, error) {
	desiredMemberCount := int(clusterSpec.MemberCount)
	desiredQuorumSize := quorumSize(desiredMemberCount)

	if len(clusterState.peers) < desiredQuorumSize {
		glog.Infof("Insufficient peers to form a quorum %d, won't proceed", quorumSize)
		return false, nil
	}

	if len(clusterState.peers) < desiredMemberCount {
		// TODO: We should relax this, but that requires etcd to support an explicit quorum setting, or we can create dummy entries

		// But ... as a special case, we can allow it through if the quorum size is the same (i.e. one less than desired)
		if quorumSize(len(clusterState.peers)) == desiredQuorumSize {
			glog.Infof("Fewer peers (%d) than desired members (%d), but quorum size is the same, so will proceed", len(clusterState.peers), desiredMemberCount)
		} else {
			glog.Infof("Insufficient peers to form full cluster %d, won't proceed", quorumSize)
			return false, nil
		}
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

	if len(proposal) < desiredMemberCount && quorumSize(len(proposal)) < quorumSize(desiredMemberCount) {
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
