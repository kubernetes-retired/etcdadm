package controller

import (
	"context"
	crypto_rand "crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	math_rand "math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/pkg/backup"
	"kope.io/etcd-manager/pkg/backupcontroller"
	"kope.io/etcd-manager/pkg/commands"
	"kope.io/etcd-manager/pkg/contextutil"
	"kope.io/etcd-manager/pkg/etcdclient"
	"kope.io/etcd-manager/pkg/locking"
	"kope.io/etcd-manager/pkg/privateapi"
)

const removeUnhealthyDeadline = time.Minute // TODO: increase

// defaultCycleInterval is the default value of EtcdController::CycleInterval
const defaultCycleInterval = 10 * time.Second

// EtcdController is the controller that runs the etcd cluster - adding & removing members, backups/restores etcd
type EtcdController struct {
	clusterName string

	// controlRefreshInterval determines how often we live-reload from the control store
	// We also refresh when we become leader, so a rolling update will force a reload
	controlRefreshInterval time.Duration

	backupStore backup.Store

	mutex sync.Mutex

	peers privateapi.Peers

	leaderLock      locking.Lock
	leaderLockGuard locking.LockGuard

	leadership *leadershipState
	peerState  map[privateapi.PeerId]*peerState

	// CycleInterval is the time to wait in between iterations of the state synchronization loop, when no progress has been made previously
	CycleInterval time.Duration

	// lastBackup is the time at which we last performed a backup (as leader)
	lastBackup time.Time

	// backupCleanup manages cleaning up old backups from the backupStore
	backupCleanup *backupcontroller.BackupCleanup

	// controlStore is the store / source of commands
	controlStore commands.Store

	// controlMutex guards the control variables beloe
	controlMutex sync.Mutex

	// controlCommands is the list of commands in the queue
	controlCommands []commands.Command
	controlLastRead time.Time

	// controlClusterSpec is the expected cluster spec, as read from the control store
	controlClusterSpec *protoetcd.ClusterSpec

	dnsSuffix string
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

// NewEtcdController is the constructor for an EtcdController
func NewEtcdController(leaderLock locking.Lock, backupStore backup.Store, controlStore commands.Store, controlRefreshInterval time.Duration, clusterName string, peers privateapi.Peers) (*EtcdController, error) {
	if clusterName == "" {
		return nil, fmt.Errorf("ClusterName is required")
	}
	m := &EtcdController{
		clusterName:            clusterName,
		backupStore:            backupStore,
		peers:                  peers,
		leaderLock:             leaderLock,
		CycleInterval:          defaultCycleInterval,
		backupCleanup:          backupcontroller.NewBackupCleanup(backupStore),
		controlStore:           controlStore,
		controlRefreshInterval: controlRefreshInterval,
	}

	return m, nil
}

// Run starts an EtcdController.  It runs indefinitely - until ctx is no longer valid.
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

func (m *EtcdController) run(ctx context.Context) (bool, error) {
	glog.V(6).Infof("starting controller iteration")

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

		if err := m.refreshControlStore(time.Duration(0)); err != nil {
			return false, fmt.Errorf("error refreshing control store after leadership change: %v", err)
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
		return false, nil
	}

	// Check that all peers have acked the leader
	{
		for _, peer := range peers {
			if !m.leadership.acked[peer.Id] {
				glog.Infof("peer %q has not acked our leadership; resigning leadership", peer)
				m.leadership = nil

				// Wait one cycle after leadership changes
				return false, nil
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
				lastEtcdHealthy: now, // We start it as healthy, so we always wait before removing it
			}
			m.peerState[privateapi.PeerId(id)] = ps
		}
		if clusterState.healthyMembers[id] != nil {
			ps.lastEtcdHealthy = now
		}
	}

	// Number of peers that are configured as part of this cluster
	configuredMembers := 0
	quarantinedMembers := 0
	for _, peer := range clusterState.peers {
		if peer.info == nil {
			continue
		}
		if peer.info.EtcdState != nil && peer.info.EtcdState.Cluster != nil {
			//// TODO: Cross-check that the configuration is the same
			//clusterConfiguration = peer.info.ClusterConfiguration

			// TODO: Cross-check that token is the same
			configuredMembers++

			if peer.info.EtcdState.Quarantined {
				quarantinedMembers++
			}
		}
	}

	if err := m.broadcastMemberMap(ctx, clusterState); err != nil {
		glog.Warningf("error broadcasting member map: %v", err)
	}

	if err := m.refreshControlStore(m.controlRefreshInterval); err != nil {
		return false, fmt.Errorf("error refreshing commands: %v", err)
	}

	isNewCluster, err := m.controlStore.IsNewCluster()
	if err != nil {
		return false, fmt.Errorf("error checking control store: %v", err)
	}
	if isNewCluster {
		glog.Infof("detected that there is no existing cluster")

		if err := m.refreshControlStore(time.Duration(0)); err != nil {
			return false, fmt.Errorf("error refreshing control store: %v", err)
		}

		clusterSpec := m.getControlClusterSpec()
		if clusterSpec != nil {
			created, err := m.createNewCluster(ctx, clusterState, clusterSpec)
			if err != nil {
				return created, err
			}
			if created {
				// Mark cluster created so we won't create it again
				if err := m.controlStore.MarkClusterCreated(); err != nil {
					return false, err
				}
			}
			return true, nil
		}
	}

	clusterSpec := m.getControlClusterSpec()
	if clusterSpec == nil {
		glog.Infof("no cluster spec set - must seed new cluster")
		return false, nil
	}
	glog.Infof("spec %v", clusterSpec)

	desiredQuorumSize := quorumSize(int(clusterSpec.MemberCount))

	restoreBackupCommand := m.getRestoreBackupCommand()
	if restoreBackupCommand != nil {
		glog.Infof("got restore-backup command: %v", restoreBackupCommand.Data)

		if restoreBackupCommand.Data().RestoreBackup == nil || restoreBackupCommand.Data().RestoreBackup.ClusterSpec == nil {
			// Should be unreachable
			return false, fmt.Errorf("RestoreBackup was not set: %v", restoreBackupCommand)
		}

		clusterSpec := restoreBackupCommand.Data().RestoreBackup.ClusterSpec
		if _, err := m.createNewCluster(ctx, clusterState, clusterSpec); err != nil {
			return false, err
		}

		return m.restoreBackupAndLiftQuarantine(ctx, clusterSpec, clusterState, restoreBackupCommand)
	}

	if len(clusterState.members) != 0 {
		if err := m.maybeBackup(ctx, clusterSpec, clusterState); err != nil {
			glog.Warningf("error during backup: %v", err)
		}
	}

	// Check if the cluster is not of the desired version
	var versionMismatch []*etcdClusterPeerInfo
	{
		for _, peer := range clusterState.peers {
			if peer.info != nil && peer.info.EtcdState != nil && peer.info.EtcdState.EtcdVersion != clusterSpec.EtcdVersion {
				glog.Infof("mismatched version for peer %v: want %q, have %q", peer.peer, clusterSpec.EtcdVersion, peer.info.EtcdState.EtcdVersion)
				versionMismatch = append(versionMismatch, peer)
			}
		}

	}

	if quarantinedMembers > 0 {
		if len(clusterState.healthyMembers) >= desiredQuorumSize && len(versionMismatch) == 0 {
			// We're ready - lift quarantine
			return m.updateQuarantine(ctx, clusterState, false)
		}
	}

	if len(clusterState.members) < int(clusterSpec.MemberCount) {
		glog.Infof("etcd has %d members registered, we want %d; will try to expand cluster", len(clusterState.members), clusterSpec.MemberCount)
		return m.addNodeToCluster(ctx, clusterSpec, clusterState)
	}

	if len(clusterState.members) == 0 {
		glog.Warningf("no members are actually running")
		return false, nil
	}

	if configuredMembers > int(clusterSpec.MemberCount) {
		return m.removeNodeFromCluster(ctx, clusterSpec, clusterState, true)
	}

	// healthy members
	if len(clusterState.healthyMembers) < int(len(clusterState.members)) {
		glog.Infof("etcd has unhealthy members")
		// TODO: Wait longer in case of a flake
		// TODO: Still backup before mutating the cluster
		return m.removeNodeFromCluster(ctx, clusterSpec, clusterState, false)
	}

	if len(versionMismatch) != 0 {
		glog.Infof("detected that we need to upgrade/downgrade etcd")

		return m.stopForUpgrade(ctx, clusterSpec, clusterState)
	}

	glog.Infof("controller loop complete")

	return false, nil
}

func (m *EtcdController) maybeBackup(ctx context.Context, clusterSpec *protoetcd.ClusterSpec, clusterState *etcdClusterState) error {
	now := time.Now()

	backupInterval := 5 * time.Minute
	shouldBackup := now.Sub(m.lastBackup) > backupInterval

	if !shouldBackup {
		return nil
	}

	backup, err := m.doClusterBackup(ctx, clusterSpec, clusterState)
	if err != nil {
		return err
	}

	glog.Infof("took backup: %v", backup)
	m.lastBackup = now

	if err := m.backupCleanup.MaybeDoBackupMaintenance(ctx); err != nil {
		glog.Warningf("error during backup cleanup: %v", err)
	}

	return nil
}

func randomToken() string {
	b := make([]byte, 16, 16)
	_, err := io.ReadFull(crypto_rand.Reader, b)
	if err != nil {
		glog.Fatalf("error generating random token: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func (m *EtcdController) broadcastMemberMap(ctx context.Context, etcdClusterState *etcdClusterState) error {
	memberMap := &protoetcd.MemberMap{}
	for _, peer := range etcdClusterState.peers {
		if peer.peer == nil || peer.peer.info == nil {
			continue
		}

		etcdState := peer.info.EtcdState
		if etcdState == nil {
			continue
		}

		nodeConfiguration := peer.info.NodeConfiguration
		if nodeConfiguration == nil {
			continue
		}

		memberInfo := &protoetcd.MemberMapInfo{
			Name: nodeConfiguration.Name,
		}

		if m.dnsSuffix != "" {
			dnsSuffix := m.dnsSuffix
			if !strings.HasPrefix(dnsSuffix, ".") {
				dnsSuffix = "." + dnsSuffix
			}
			memberInfo.Dns = nodeConfiguration.Name + dnsSuffix
		}

		for _, a := range peer.peer.info.Endpoints {
			ip := a
			colonIndex := strings.Index(ip, ":")
			if colonIndex != -1 {
				ip = ip[:colonIndex]
			}
			memberInfo.Addresses = append(memberInfo.Addresses, ip)
		}

		memberMap.Members = append(memberMap.Members, memberInfo)
	}

	// TODO: optimize this
	glog.Infof("sending member map to all peers: %v", memberMap)

	for _, peer := range etcdClusterState.peers {
		updateEndpointsRequest := &protoetcd.UpdateEndpointsRequest{
			MemberMap: memberMap,
		}

		_, err := peer.peer.rpcUpdateEndpoints(ctx, updateEndpointsRequest)
		if err != nil {
			glog.Warningf("peer %s failed to update endpoints: %v", peer.peer.Id, err)
		}
	}

	return nil
}

// updateClusterState queries each peer (including ourselves) for information about the desired state of the world
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
		if p.info.NodeConfiguration == nil {
			continue
		}
		if p.info.EtcdState == nil {
			continue
		}

		clientUrls := p.info.NodeConfiguration.ClientUrls
		if p.info.EtcdState.Quarantined {
			clientUrls = p.info.NodeConfiguration.QuarantinedClientUrls
		}
		if len(p.info.NodeConfiguration.ClientUrls) == 0 {
			continue
		}

		etcdClient, err := etcdclient.NewClient(p.info.EtcdState.EtcdVersion, clientUrls)
		if err != nil {
			glog.Warningf("unable to reach member %s: %v", p, err)
			continue
		}
		members, err := etcdClient.ListMembers(ctx)
		etcdclient.LoggedClose(etcdClient)
		if err != nil {
			glog.Warningf("unable to reach member %s: %v", p, err)
			continue
		}

		clusterState.members = make(map[EtcdMemberId]*etcdclient.EtcdProcessMember)
		for _, m := range members {
			// Note that members don't necessarily have names, when they are added but not yet merged
			memberID := EtcdMemberId(m.ID)
			if memberID == "" {
				glog.Fatalf("etcd member did not have ID: %v", m)
			}
			clusterState.members[memberID] = m
		}
		break
	}

	// Query each cluster member to see if it is healthy (and collect the version it is running)
	clusterState.healthyMembers = make(map[EtcdMemberId]*etcdclient.EtcdProcessMember)
	//clusterState.versions = make(map[EtcdMemberId]*version.Versions)
	for id, member := range clusterState.members {
		etcdClient, err := member.NewClient()
		if err != nil {
			glog.Warningf("health-check unable to reach member %s: %v", id, err)
			continue
		}

		_, err = etcdClient.ListMembers(ctx)
		etcdclient.LoggedClose(etcdClient)
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

func (m *EtcdController) addNodeToCluster(ctx context.Context, clusterSpec *protoetcd.ClusterSpec, clusterState *etcdClusterState) (bool, error) {
	var peersMissingFromEtcd []*etcdClusterPeerInfo
	var idlePeers []*etcdClusterPeerInfo
	for _, peer := range clusterState.peers {
		if peer.info != nil && peer.info.EtcdState != nil && peer.info.EtcdState.Cluster != nil {
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

	// We need to start etcd on a new node
	if len(idlePeers) != 0 {
		// Force a backup first
		if _, err := m.doClusterBackup(ctx, clusterSpec, clusterState); err != nil {
			return false, fmt.Errorf("failed to backup (before adding peer): %v", err)
		}

		peer := idlePeers[math_rand.Intn(len(idlePeers))]
		glog.Infof("will try to start new peer: %v", peer)

		clusterToken := ""
		etcdVersion := ""
		for _, peer := range clusterState.peers {
			if peer.info != nil && peer.info.EtcdState != nil && peer.info.EtcdState.Cluster != nil && peer.info.EtcdState.Cluster.ClusterToken != "" {
				clusterToken = peer.info.EtcdState.Cluster.ClusterToken
				etcdVersion = peer.info.EtcdState.EtcdVersion
			}
		}
		if clusterToken == "" {
			// Should be unreachable
			return false, fmt.Errorf("unable to determine cluster token")
		}

		var nodes []*protoetcd.EtcdNode
		for _, member := range clusterState.members {
			node := &protoetcd.EtcdNode{
				Name:       member.Name,
				ClientUrls: member.ClientURLs,
				PeerUrls:   member.PeerURLs,
			}
			nodes = append(nodes, node)
		}

		{
			node := proto.Clone(peer.info.NodeConfiguration).(*protoetcd.EtcdNode)
			nodes = append(nodes, node)
		}

		{
			joinClusterRequest := &protoetcd.JoinClusterRequest{
				Header:       m.buildHeader(),
				Phase:        protoetcd.Phase_PHASE_PREPARE,
				ClusterToken: clusterToken,
				EtcdVersion:  etcdVersion,
				Nodes:        nodes,
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
				Header:       m.buildHeader(),
				Phase:        protoetcd.Phase_PHASE_JOIN_EXISTING,
				ClusterToken: clusterToken,
				EtcdVersion:  etcdVersion,
				Nodes:        nodes,
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

// doClusterBackup triggers a backup of etcd, on any healthy cluster member
func (m *EtcdController) doClusterBackup(ctx context.Context, clusterSpec *protoetcd.ClusterSpec, clusterState *etcdClusterState) (*protoetcd.DoBackupResponse, error) {
	for _, member := range clusterState.healthyMembers {
		peer := clusterState.FindPeer(member)
		if peer == nil {
			glog.Warningf("unable to find peer for member %v", member)
			continue
		}

		info := &protoetcd.BackupInfo{
			ClusterSpec: clusterSpec,
		}
		doBackupRequest := &protoetcd.DoBackupRequest{
			Header:  m.buildHeader(),
			Storage: m.backupStore.Spec(),
			Info:    info,
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

func (m *EtcdController) removeNodeFromCluster(ctx context.Context, clusterSpec *protoetcd.ClusterSpec, clusterState *etcdClusterState, removeHealthy bool) (bool, error) {
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

	//var peer *etcdClusterState
	//for _, p := range clusterState.peers {
	//	if p.info != nil && p.info.Id == victim.Id {
	//		peer = p.peer
	//	}
	//}
	//if peer == nil {
	//	return false, fmt.Errorf("unable to find peer for member %v", victim)
	//}

	// Force a backup first
	if _, err := m.doClusterBackup(ctx, clusterSpec, clusterState); err != nil {
		return false, fmt.Errorf("failed to backup (before adding peer): %v", err)
	}

	glog.Infof("removing node from etcd cluster: %v", victim)

	err := clusterState.etcdRemoveMember(ctx, victim)
	if err != nil {
		return false, fmt.Errorf("failed to remove member %q: %v", victim, err)
	}

	// TODO: Need to look for peers that are running etcd but aren't in the cluster
	// (we could do it here, but we want to do it as part of the control loop for safety)
	glog.Errorf("TODO: Remove peers that aren't active in the cluster")

	return true, nil
}

// quorumSize computes the number of nodes in a quorum, for a given cluster size.
// quorumSize = (N / 2) + 1
func quorumSize(desiredMemberCount int) int {
	return (desiredMemberCount / 2) + 1
}

// createNewCluster starts a new etcd cluster.
// It tries to identify a quorum of nodes, and if found will instruct each to join the cluster.
func (m *EtcdController) createNewCluster(ctx context.Context, clusterState *etcdClusterState, clusterSpec *protoetcd.ClusterSpec) (bool, error) {
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
			// We have identified enough members to form a cluster
			break
		}
	}

	if len(proposal) < desiredMemberCount && quorumSize(len(proposal)) < quorumSize(desiredMemberCount) {
		glog.Fatalf("Need to add dummy peers to force quorum size :-(")
	}

	var proposedNodes []*protoetcd.EtcdNode
	for _, p := range proposal {
		node := proto.Clone(p.info.NodeConfiguration).(*protoetcd.EtcdNode)
		proposedNodes = append(proposedNodes, node)
	}

	// Stop any running etcd
	for _, p := range clusterState.peers {
		peer := p.peer

		if p.info != nil && p.info.EtcdState != nil {
			request := &protoetcd.StopEtcdRequest{
				Header: m.buildHeader(),
			}
			response, err := peer.rpcStopEtcd(ctx, request)
			if err != nil {
				return false, fmt.Errorf("error stopping etcd peer %q: %v", peer.Id, err)
			}
			glog.Infof("stopped etcd on peer %q: %v", peer.Id, response)
		}
	}

	glog.Infof("starting new etcd cluster with %s", proposal)

	for _, p := range proposal {
		// Note the we may send the message to ourselves
		joinClusterRequest := &protoetcd.JoinClusterRequest{
			Header:       m.buildHeader(),
			Phase:        protoetcd.Phase_PHASE_PREPARE,
			ClusterToken: clusterToken,
			EtcdVersion:  clusterSpec.EtcdVersion,
			Nodes:        proposedNodes,
		}

		joinClusterResponse, err := p.peer.rpcJoinCluster(ctx, joinClusterRequest)
		if err != nil {
			// TODO: Send a CANCEL message for anything PREPAREd?  (currently we rely on a slow timeout)
			return false, fmt.Errorf("error from JoinClusterRequest from peer %q: %v", p.peer, err)
		}
		glog.V(2).Infof("JoinClusterResponse: %s", joinClusterResponse)
	}

	for _, p := range proposal {
		// Note the we may send the message to ourselves
		joinClusterRequest := &protoetcd.JoinClusterRequest{
			Header:       m.buildHeader(),
			Phase:        protoetcd.Phase_PHASE_INITIAL_CLUSTER,
			ClusterToken: clusterToken,
			EtcdVersion:  clusterSpec.EtcdVersion,
			Nodes:        proposedNodes,
		}

		joinClusterResponse, err := p.peer.rpcJoinCluster(ctx, joinClusterRequest)
		if err != nil {
			// TODO: Send a CANCEL message for anything PREPAREd?  (currently we rely on a slow timeout)
			return false, fmt.Errorf("error from JoinClusterRequest from peer %q: %v", p.peer, err)
		}
		glog.V(2).Infof("JoinClusterResponse: %s", joinClusterResponse)
	}

	// Write cluster spec to etcd
	if err := m.writeClusterSpec(ctx, clusterState, clusterSpec); err != nil {
		return false, err
	}

	return true, nil
}

func (m *EtcdController) buildHeader() *protoetcd.CommonRequestHeader {
	return &protoetcd.CommonRequestHeader{
		LeadershipToken: m.leadership.token,
		ClusterName:     m.clusterName,
	}
}
