package etcd

import (
	crypto_rand "crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"sync"
	"time"
	"io/ioutil"
	"os"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	"kope.io/etcd-manager/pkg/privateapi"
)

type EtcdManager struct {
	mutex sync.Mutex

	model   EtcdCluster
	baseDir string

	process *etcdProcess

	me    privateapi.PeerId
	peers privateapi.Peers
}

func (s *EtcdServer) NewEtcdManager(model *EtcdCluster, baseDir string) (*EtcdManager, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if model.DesiredClusterSize < 1 {
		return nil, fmt.Errorf("DesiredClusterSize must be > 1")
	}
	if model.ClusterName == "" {
		return nil, fmt.Errorf("ClusterName is required")
	}
	if s.etcdClusters[model.ClusterName] != nil {
		return nil, fmt.Errorf("cluster %q already registered", model.ClusterName)
	}
	m := &EtcdManager{
		model:   *model,
		baseDir: baseDir,
		peers:   s.peerServer,
		me:      s.peerServer.MyPeerId(),
	}
	s.etcdClusters[model.ClusterName] = m
	return m, nil
}

func (m *EtcdManager) Run() {
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

func (m *EtcdManager) run(ctx context.Context) (bool, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	desiredMemberCount := int(m.model.DesiredClusterSize)

	peers := m.peers.Peers()
	sort.SliceStable(peers, func(i, j int) bool {
		return peers[i].Id < peers[j].Id
	})
	glog.Infof("peers: %s", peers)

	var me *privateapi.PeerInfo
	for _, peer := range peers {
		if peer.Id == string(m.me) {
			me = peer
		}
	}
	if me == nil {
		return false, fmt.Errorf("cannot find self %q in list of peers %s", m.me, peers)
	}

	if m.process == nil {
		return m.stepStartCluster(me, peers)
	}

	if m.process == nil {
		return false, fmt.Errorf("unexpected state in control loop - no etcd process")
	}

	members, err := m.process.listMembers()
	if err != nil {
		return false, fmt.Errorf("error querying members from etcd: %v", err)
	}

	// TODO: Only if we are leader

	actualMemberCount := len(members)
	if actualMemberCount < desiredMemberCount {
		glog.Infof("will try to add %d members; existing members: %s", desiredMemberCount-actualMemberCount, members)

		// TODO: Randomize peer selection?

		membersByName := make(map[string]*etcdProcessMember)
		for _, member := range members {
			membersByName[member.Name] = member
		}

		for _, peer := range peers {
			if membersByName[peer.Id] != nil {
				continue
			}

			cluster := &EtcdCluster{
				ClusterToken: m.process.Cluster.ClusterToken,
				ClusterName:  m.process.Cluster.ClusterName,
			}
			for _, member := range members {
				etcdNode := &EtcdNode{
					Name:       member.Name,
					PeerUrls:   member.PeerURLs,
					ClientUrls: member.ClientURLs,
				}
				cluster.Nodes = append(cluster.Nodes, etcdNode)
			}
			joinClusterRequest := &JoinClusterRequest{
				Cluster: cluster,
			}
			peerGrpcClient, err := m.peers.GetPeerClient(privateapi.PeerId(peer.Id))
			if err != nil {
				return false, fmt.Errorf("error getting peer client %q: %v", peer.Id, err)
			}
			peerClient := NewEtcdManagerServiceClient(peerGrpcClient)
			joinClusterResponse, err := peerClient.JoinCluster(ctx, joinClusterRequest)
			if err != nil {
				return false, fmt.Errorf("error from JoinClusterRequest from peer %q: %v", peer.Id, err)
			}
			if joinClusterResponse.Node == nil {
				return false, fmt.Errorf("JoingClusterResponse from peer %q did not include peer info", peer.Id, joinClusterResponse)
			}
			peerURLs := joinClusterResponse.Node.PeerUrls
			//var peerURLs []string
			//for _, address := range peer.Addresses {
			//	peerURL := fmt.Sprintf("http://%s:%d", address, m.model.PeerPort)
			//	peerURLs = append(peerURLs, peerURL)
			//}
			glog.Infof("will try to add member %s @ %s", peer.Id, peerURLs)
			member, err := m.process.addMember(peer.Id, peerURLs)
			if err != nil {
				return false, fmt.Errorf("failed to add peer %s %s: %v", peer.Id, peerURLs, err)
			}
			glog.Infof("Added member: %s", member)
			actualMemberCount++
			if actualMemberCount == desiredMemberCount {
				break
			}
		}
	}

	// TODO: Eventually replace unhealthy members?

	return false, nil
}

func randomToken() string {
	b := make([]byte, 16, 16)
	_, err := io.ReadFull(crypto_rand.Reader, b)
	if err != nil {
		glog.Fatalf("error generating random token: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func (m *EtcdManager) joinCluster(ctx context.Context, request *JoinClusterRequest) (*JoinClusterResponse, error) {
	glog.Infof("got joinCluster request: %s", request)

	m.mutex.Lock()
	defer m.mutex.Unlock()

	peers := m.peers.Peers()
	sort.SliceStable(peers, func(i, j int) bool {
		return peers[i].Id < peers[j].Id
	})
	glog.Infof("peers: %s", peers)

	var me *privateapi.PeerInfo
	for _, peer := range peers {
		if peer.Id == string(m.me) {
			me = peer
		}
	}
	if me == nil {
		return nil, fmt.Errorf("cannot find self %q in list of peers %s", m.me, peers)
	}

	if m.process != nil {
		members, err := m.process.listMembers()
		if err != nil {
			// TODO: We probably want to retry here, as if we have an unhealthy cluster we want to kill it and join a better one
			return nil, fmt.Errorf("error querying members from etcd: %v", err)
		}
		if len(request.Cluster.Nodes) < len(members) {
			// TODO: Total ordering, probably using peerIds or the cluster token
			return nil, fmt.Errorf("will not join a smaller cluster")
		}

		if err := m.process.Stop(); err != nil {
			return nil, fmt.Errorf("unable to stop etcd: %v", err)
		}

		m.process = nil
	}

	clusterToken := request.Cluster.ClusterToken

	dataDir := filepath.Join(m.baseDir, clusterToken)

	meNode := &EtcdNode{
		Name: me.Id,
	}
	for _, a := range me.Addresses {
		peerUrl := fmt.Sprintf("http://%s:%d", a, m.model.PeerPort)
		meNode.PeerUrls = append(meNode.PeerUrls, peerUrl)
	}
	for _, a := range me.Addresses {
		clientUrl := fmt.Sprintf("http://%s:%d", a, m.model.ClientPort)
		meNode.ClientUrls = append(meNode.ClientUrls, clientUrl)
	}
	p := &etcdProcess{
		CreateNewCluster: false,
		BinDir:           "/home/justinsb/apps/etcd2/etcd-v2.2.1-linux-amd64",
		DataDir:          dataDir,

		Cluster: &EtcdCluster{
			PeerPort:     m.model.PeerPort,
			ClientPort:   m.model.ClientPort,
			ClusterName:  m.model.ClusterName,
			ClusterToken: clusterToken,
			Me:           meNode,
			Nodes:        []*EtcdNode{meNode},
		},
	}

	for _, node := range request.Cluster.Nodes {
		p.Cluster.Nodes = append(p.Cluster.Nodes, node)
	}

	err := p.Start()
	if err != nil {
		return nil, fmt.Errorf("error starting etcd: %v", err)
	}
	m.process = p

	glog.Infof("Started cluster")

	response := &JoinClusterResponse{
		Node: meNode,
	}
	return response, nil
}

func (m *EtcdManager) stepStartCluster(me *privateapi.PeerInfo, peers []*privateapi.PeerInfo) (bool, error) {
	desiredMemberCount := int(m.model.DesiredClusterSize)
	quorumSize := (desiredMemberCount / 2) + 1

	// TODO: Query the others to make sure they are OK with joining?

	// We will start a single member cluster, with us as the node, and then we will try to join others to us
	var dirs []string
	files, err := ioutil.ReadDir(m.baseDir)
	if err != nil {
		return false, fmt.Errorf("error listing contents of %s: %v", m.baseDir, err)
	}
	for _, f := range files {
		name := f.Name()
		if f.IsDir() {
			dirs = append(dirs, name)
		}
	}

	if len(dirs) > 1 {
		// If there are multiple dirs see if we can whittle the list using the state file
		var activeDirs []string
		for _, dir := range dirs {
			p := filepath.Join(m.baseDir, dir+".etcdmanager")
			_, err := os.Stat(p)
			if err != nil {
				if !os.IsNotExist(err) {
					return false, fmt.Errorf("error checking for stat of %s: %v", p, err)
				}
			} else {
				activeDirs = append(activeDirs)
			}
		}
		if len(activeDirs) != 0 {
			dirs = activeDirs
		}
	}

	var clusterToken string
	createNewCluster := false

	if len(dirs) > 1 {
		return false, fmt.Errorf("unable to determine active cluster version from %s", dirs)
	} else if len(dirs) == 1 {
		// Existing cluster
		clusterToken = dirs[0]
	} else {
		// Consider starting a new cluster?

		if len(peers) < quorumSize {
			glog.Infof("Insufficient peers to form a quorum %d, won't proceed", quorumSize)
			return false, nil
		}

		if peers[0].Id != me.Id {
			// We are not the leader, we won't initiate
			glog.Infof("we are not leader, won't initiate cluster creation")
			return false, nil
		}

		createNewCluster = true
		clusterToken = randomToken()
	}

	dataDir := filepath.Join(m.baseDir, clusterToken)

	{
		p := filepath.Join(m.baseDir, clusterToken+".etcdmanager")
		if err := ioutil.WriteFile(p, []byte{}, 0755); err != nil {
			return false, fmt.Errorf("error writing state file %s: %q", p)
		}
	}

	meNode := &EtcdNode{
		// TODO: Include the cluster token (or a portion of it) in the name?
		Name: me.Id,
	}
	for _, a := range me.Addresses {
		peerUrl := fmt.Sprintf("http://%s:%d", a, m.model.PeerPort)
		meNode.PeerUrls = append(meNode.PeerUrls, peerUrl)
	}
	for _, a := range me.Addresses {
		clientUrl := fmt.Sprintf("http://%s:%d", a, m.model.ClientPort)
		meNode.ClientUrls = append(meNode.ClientUrls, clientUrl)
	}
	p := &etcdProcess{
		// We always create new cluster, because etcd will ignore if the cluster exists
		// TODO: Should we do better?
		CreateNewCluster: createNewCluster,
		BinDir:           "/home/justinsb/apps/etcd2/etcd-v2.2.1-linux-amd64",
		DataDir:          dataDir,

		Cluster: &EtcdCluster{
			PeerPort:     m.model.PeerPort,
			ClientPort:   m.model.ClientPort,
			ClusterName:  m.model.ClusterName,
			ClusterToken: clusterToken,
			Me:           meNode,
			Nodes:        []*EtcdNode{meNode},
		},
	}

	if err := p.Start(); err != nil {
		return false, fmt.Errorf("error starting etcd: %v", err)
	}
	m.process = p

	// TODO: Wait till etcd has started and then write state file?

	return true, nil
}
