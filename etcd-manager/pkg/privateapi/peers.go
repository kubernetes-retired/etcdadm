package privateapi

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"
	"google.golang.org/grpc"

	"kope.io/etcd-manager/pkg/contextutil"
	"kope.io/etcd-manager/pkg/privateapi/discovery"
)

const defaultPingInterval = time.Second * 5
const defaultHealthyTimeout = time.Minute

// defaultDiscoveryPollInterval is the default value of Server::DiscoveryPollInterval
const defaultDiscoveryPollInterval = time.Minute

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
	discoveryNode discovery.Node

	// Addresses from GRPC
	lastInfo     *PeerInfo
	lastPingTime time.Time

	conn *grpc.ClientConn

	// DiscoveryPollInterval is the frequency with which we perform peer discovery
	DiscoveryPollInterval time.Duration
}

func (s *Server) runDiscovery(ctx context.Context) {
	contextutil.Forever(ctx, s.DiscoveryPollInterval, func() {
		err := s.runDiscoveryOnce()
		if err != nil {
			glog.Warningf("unexpected error from peer intercommunications: %v", err)
		}
	})
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
		glog.Infof("found new candidate peer from ping: %s %v", id, request.Info)
		existing = &peer{
			server: s,
			id:     id,
		}
		existing.updatePeerInfo(request.Info)
		s.peers[id] = existing
		go existing.Run(s.context, s.PingInterval)
	} else {
		existing.updatePeerInfo(request.Info)
	}

}

func (s *Server) updateFromDiscovery(discoveryNode discovery.Node) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	id := PeerId(discoveryNode.ID)

	existing := s.peers[id]
	if existing == nil {
		glog.Infof("found new candidate peer from discovery: %s %v", id, discoveryNode.Endpoints)
		existing = &peer{
			server: s,
			id:     id,
		}
		existing.updateFromDiscovery(discoveryNode)
		s.peers[id] = existing
		go existing.Run(s.context, s.PingInterval)
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

func (p *peer) updateFromDiscovery(discoveryNode discovery.Node) {
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

func (p *peer) status(healthyTimeout time.Duration) (*PeerInfo, bool) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.lastInfo == nil {
		return nil, false
	}

	now := time.Now()
	if now.Sub(p.lastPingTime) > healthyTimeout {
		return nil, false
	}

	return p.lastInfo, true
}

func (p *peer) Run(ctx context.Context, pingInterval time.Duration) {
	contextutil.Forever(ctx, pingInterval, func() {
		err := p.sendPings(ctx, pingInterval)
		if err != nil {
			glog.Warningf("unexpected error from peer intercommunications: %v", err)
		}
	})
}

func (p *peer) sendPings(ctx context.Context, pingInterval time.Duration) error {
	conn, err := p.connect()
	if err != nil {
		return err
	}
	if conn == nil {
		return fmt.Errorf("unable to connect to peer %s", p.id)
	}

	client := NewClusterServiceClient(conn)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

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

		contextutil.Sleep(ctx, pingInterval)
	}
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

	endpoints := make(map[string]bool)
	{
		p.mutex.Lock()
		for _, endpoint := range p.discoveryNode.Endpoints {
			endpoints[endpoint.Endpoint] = true
		}

		if p.lastInfo != nil {
			for _, endpoint := range p.lastInfo.Endpoints {
				endpoints[endpoint] = true
			}
		}

		p.mutex.Unlock()
	}
	for endpoint := range endpoints {
		conn, err := grpc.Dial(endpoint, opts...)
		if err != nil {
			glog.Warningf("unable to connect to discovered peer %s: %v", endpoint, err)
			continue
		}

		client := NewClusterServiceClient(conn)
		context := context.Background()
		request := &PingRequest{
			Info: &p.server.myInfo,
		}
		response, err := client.Ping(context, request)
		if err != nil {
			glog.Warningf("unable to talk to discovered peer %s: %v", endpoint, err)
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

	glog.Infof("was not able to connect to peer %s: %v", p.discoveryNode.ID, endpoints)
	return nil, nil
}

var _ Peers = &Server{}

func (s *Server) Peers() []*PeerInfo {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	var infos []*PeerInfo

	for _, peer := range s.peers {
		lastInfo, status := peer.status(s.HealthyTimeout)
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

	_, status := peer.status(s.HealthyTimeout)
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
