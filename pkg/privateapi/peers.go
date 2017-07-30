package privateapi

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"
	"google.golang.org/grpc"
)

const HealthyTimeout = time.Minute

type PeerId string

type Peers interface {
	Peers() []*PeerInfo
	MyPeerId() PeerId
	GetPeerClient(peerId PeerId) (*grpc.ClientConn, error)
	BecomeLeader(ctx context.Context) ([]PeerId, string, error)
	IsLeader(token string) bool
}

type peer struct {
	id     PeerId
	server *Server

	mutex sync.Mutex

	// Information from discovery.  Should not be considered authoritative
	discoveryNode DiscoveryNode

	// Addresses from GRPC
	lastInfo     *PeerInfo
	lastPingTime time.Time

	conn *grpc.ClientConn
}

func (s *Server) runDiscovery() {
	for {
		err := s.runDiscoveryOnce()
		if err != nil {
			glog.Warningf("unexpected error from peer intercommunications: %v", err)
		}

		time.Sleep(1 * time.Minute)
	}
}

func (s *Server) updateFromPingRequest(request *PingRequest) {
	if request.Info == nil || request.Info.Id == "" {
		glog.Warningf("ignoring ping with no id: %s", request)
		return
	}
	id := PeerId(request.Info.Id)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	existing := s.peers[id]
	if existing == nil {
		glog.Infof("discovery found new candidate peer: %s", id)
		existing = &peer{
			server: s,
			id:     id,
		}
		s.peers[id] = existing
		existing.updatePeerInfo(request.Info)
		go existing.Run()
	} else {
		existing.updatePeerInfo(request.Info)
	}

}

func (s *Server) updateFromDiscovery(discoveryNode DiscoveryNode) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	id := PeerId(discoveryNode.ID)

	existing := s.peers[id]
	if existing == nil {
		glog.Infof("discovery found new candidate peer: %s", id)
		existing = &peer{
			server: s,
			id:     id,
		}
		s.peers[id] = existing
		existing.updateFromDiscovery(discoveryNode)
		go existing.Run()
	} else {
		existing.updateFromDiscovery(discoveryNode)
	}
}

func (s *Server) runDiscoveryOnce() error {
	nodes, err := s.discovery.Poll()
	if err != nil {
		return fmt.Errorf("error during peer discovery: %v", err)
	}

	for k := range nodes {
		s.updateFromDiscovery(nodes[k])
	}

	return nil
}

func (p *peer) updateFromDiscovery(discoveryNode DiscoveryNode) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.discoveryNode = discoveryNode
}

func (p *peer) updatePeerInfo(peerInfo *PeerInfo) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if PeerId(peerInfo.Id) != p.id {
		// TODO: We should probably keep a map by ip & socket
		glog.Warningf("ignoring peer info with unexpected identity: %q vs %q", peerInfo.Id, p.id)
		return
	}

	p.lastInfo = peerInfo
	p.lastPingTime = time.Now()
}

func (p *peer) status() (*PeerInfo, bool) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.lastInfo == nil {
		return nil, false
	}

	now := time.Now()
	if now.Sub(p.lastPingTime) > HealthyTimeout {
		return nil, false
	}

	return p.lastInfo, true
}

func (p *peer) Run() {
	for {
		err := p.runOnce()
		if err != nil {
			glog.Warningf("unexpected error from peer intercommunications: %v", err)
		}

		time.Sleep(1 * time.Minute)
	}
}

func (p *peer) runOnce() error {
	conn, err := p.connect()
	if err != nil {
		return err
	}
	if conn == nil {
		return fmt.Errorf("unable to connect to peer %s", p.id)
	}

	client := NewClusterServiceClient(conn)
	for {
		context := context.Background()
		request := &PingRequest{
			Info: &p.server.myInfo,
		}
		response, err := client.Ping(context, request)
		glog.V(8).Infof("got ping response from %s: %v", p.id, response)
		if err != nil {
			return fmt.Errorf("error pinging %s: %v", p.id, err)
		}

		p.updatePeerInfo(response.Info)

		time.Sleep(10 * time.Second)
	}
	return nil
}

func (p *peer) connect() (*grpc.ClientConn, error) {
	var conn *grpc.ClientConn

	{
		p.mutex.Lock()
		conn = p.conn
		p.mutex.Unlock()

		if conn != nil {
			return conn, nil
		}
	}

	var opts []grpc.DialOption
	//if *tls {
	//	var sn string
	//	if *serverHostOverride != "" {
	//		sn = *serverHostOverride
	//	}
	//	var creds credentials.TransportCredentials
	//	if *caFile != "" {
	//		var err error
	//		creds, err = credentials.NewClientTLSFromFile(*caFile, sn)
	//		if err != nil {
	//			grpclog.Fatalf("Failed to create TLS credentials %v", err)
	//		}
	//	} else {
	//		creds = credentials.NewClientTLSFromCert(nil, sn)
	//	}
	//	opts = append(opts, grpc.WithTransportCredentials(creds))
	//} else {
	opts = append(opts, grpc.WithInsecure())
	//}

	for _, address := range p.discoveryNode.Addresses {
		conn, err := grpc.Dial(address.IP, opts...)
		if err != nil {
			glog.Warningf("unable to connect to discovered peer %s: %v", address.IP, err)
			continue
		}

		client := NewClusterServiceClient(conn)
		context := context.Background()
		request := &PingRequest{
			Info: &p.server.myInfo,
		}
		response, err := client.Ping(context, request)
		if err != nil {
			glog.Warningf("unable to talk to discovered peer %s: %v", address.IP, err)
			conn.Close()
			continue
		}

		glog.V(8).Infof("got ping response from %s: %v", p.id, response)
		p.updatePeerInfo(response.Info)

		{
			p.mutex.Lock()
			if p.conn != nil {
				conn.Close()
				conn = p.conn
			} else {
				p.conn = conn
			}
			p.mutex.Unlock()
		}

		return conn, nil
	}

	return nil, nil
}

var _ Peers = &Server{}

func (s *Server) Peers() []*PeerInfo {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	var infos []*PeerInfo

	for _, peer := range s.peers {
		lastInfo, status := peer.status()
		if status == false {
			continue
		}
		infos = append(infos, lastInfo)
	}

	return infos
}

func (s *Server) GetPeerClient(peerId PeerId) (*grpc.ClientConn, error) {
	s.mutex.Lock()
	peer := s.peers[peerId]
	if peer == nil {
		s.mutex.Unlock()
		return nil, fmt.Errorf("peer %q not known", peerId)
	}
	s.mutex.Unlock()

	_, status := peer.status()
	if status == false {
		return nil, fmt.Errorf("peer %q is not healthy", peerId)
	}

	conn, err := peer.connect()
	if err != nil {
		return nil, fmt.Errorf("error connecting to peer %q: %v", peerId, err)
	}
	if conn == nil {
		return nil, fmt.Errorf("unable to connect to peer %q", peerId)
	}
	return conn, nil
}

func (s *Server) MyPeerId() PeerId {
	return PeerId(s.myInfo.Id)
}
