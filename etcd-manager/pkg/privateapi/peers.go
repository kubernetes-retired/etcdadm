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

package privateapi

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"k8s.io/klog/v2"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/contextutil"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/privateapi/discovery"
)

const defaultPingInterval = time.Second * 5
const defaultHealthyTimeout = time.Minute

type PeerId string

type Peers interface {
	Peers() []*PeerInfo
	MyPeerId() PeerId
	GetPeerClient(peerId PeerId) (*grpc.ClientConn, error)
	BecomeLeader(ctx context.Context) ([]PeerId, string, error)
	AssertLeadership(ctx context.Context, leadershipToken string) error
	IsLeader(token string) bool
}

type peer struct {
	id     PeerId
	server *Server

	mutex sync.Mutex

	// Information from discovery.  Should not be considered authoritative
	discoveryNode discovery.Node

	// defaultPort is the port to use when one is not specified, particularly for discovery
	defaultPort int

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
			klog.Warningf("unexpected error from peer intercommunications: %v", err)
		}
	})
}

func (s *Server) updateFromPingRequest(request *PingRequest) {
	if request.Info == nil || request.Info.Id == "" {
		klog.Warningf("ignoring ping with no id: %s", request)
		return
	}
	id := PeerId(request.Info.Id)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	existing := s.peers[id]
	if existing == nil {
		klog.Infof("found new candidate peer from ping: %s %v", id, request.Info)
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
		klog.Infof("found new candidate peer from discovery: %s %v", id, discoveryNode.Endpoints)
		existing = &peer{
			server:      s,
			id:          id,
			defaultPort: s.defaultPort,
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

	if s.dnsSuffix != "" {
		dnsSuffix := s.dnsSuffix
		if !strings.HasPrefix(dnsSuffix, ".") {
			dnsSuffix = "." + dnsSuffix
		}

		dnsFallbacks := make(map[string][]net.IP)
		for _, node := range nodes {
			dnsName := node.ID + dnsSuffix

			var ips []net.IP
			for _, e := range node.Endpoints {
				ip := net.ParseIP(e.IP)
				if ip != nil {
					ips = append(ips, ip)
				} else {
					klog.Warningf("skipping endpoint that cannot be parsed as IP: %q", e.IP)
				}
			}
			dnsFallbacks[dnsName] = ips
		}

		if err := s.dnsProvider.AddFallbacks(dnsFallbacks); err != nil {
			klog.Warningf("error registering dns fallbacks: %v", err)
		}
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
		klog.Warningf("ignoring peer info with unexpected identity: %q vs %q", peerInfo.Id, p.id)
		return
	}

	if p.lastInfo != nil {
		// TODO: Consider discovery?  Use discovery only as a fallback?
		oldEndpoints := make(map[string]bool)
		for _, endpoint := range p.lastInfo.Endpoints {
			oldEndpoints[endpoint] = true
		}
		newEndpoints := make(map[string]bool)
		for _, endpoint := range peerInfo.Endpoints {
			newEndpoints[endpoint] = true
		}
		if !reflect.DeepEqual(oldEndpoints, newEndpoints) {
			klog.Infof("peer %s changed endpoints; will invalidate connection", peerInfo.Id)
			if p.conn != nil {
				p.conn.Close()
				p.conn = nil
			}
		}
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
			klog.Warningf("unexpected error from peer intercommunications: %v", err)
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
		klog.V(8).Infof("got ping response from %s: %v", p.id, response)
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
		if p.conn != nil {
			state := p.conn.GetState()
			switch state {
			case connectivity.TransientFailure, connectivity.Shutdown:
				klog.Warningf("closing grpc connection to peer %s in state %s", p.id, state)
				p.conn.Close()
				p.conn = nil
			default:
				conn = p.conn
			}
		}
		p.mutex.Unlock()

		if conn != nil {
			return conn, nil
		}

	}

	var opts []grpc.DialOption
	if p.server.clientTLSConfig != nil {
		cfg := p.server.clientTLSConfig.Clone()
		cfg.ServerName = "etcd-manager-server-" + string(p.id)
		creds := credentials.NewTLS(cfg)

		klog.Infof("connecting to peer %q with TLS policy, servername=%q", p.id, cfg.ServerName)

		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		klog.Warningf("connecting to peer %q insecurely", p.id)
		opts = append(opts, grpc.WithInsecure())
	}

	opts = append(opts, grpc.WithBackoffMaxDelay(10*time.Second))

	endpoints := make(map[string]bool)
	{
		p.mutex.Lock()
		for _, endpoint := range p.discoveryNode.Endpoints {
			port := endpoint.Port
			if port == 0 {
				port = p.defaultPort
			}
			s := net.JoinHostPort(endpoint.IP, strconv.Itoa(port))
			endpoints[s] = true
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
			klog.Warningf("unable to connect to discovered peer %s: %v", endpoint, err)
			continue
		}

		client := NewClusterServiceClient(conn)
		context := context.Background()
		request := &PingRequest{
			Info: &p.server.myInfo,
		}
		response, err := client.Ping(context, request)
		if err != nil {
			klog.Warningf("unable to grpc-ping discovered peer %s: %v", endpoint, err)
			conn.Close()
			continue
		}

		klog.V(8).Infof("got ping response from %s: %v", p.id, response)
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

	klog.Infof("was not able to connect to peer %s: %v", p.discoveryNode.ID, endpoints)
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
