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
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"k8s.io/klog"
	"kope.io/etcd-manager/pkg/dns"
	"kope.io/etcd-manager/pkg/privateapi/discovery"
)

type Server struct {
	myInfo     PeerInfo
	grpcServer *grpc.Server

	clientTLSConfig *tls.Config

	discovery discovery.Interface

	// defaultPort is the port to use when one is not specified
	defaultPort int

	mutex      sync.Mutex
	peers      map[PeerId]*peer
	leadership *leadership

	// context is the context bounding the lifetime of this Server
	context context.Context

	// DiscoveryPollInterval is the interval with which we request peers from discovery
	DiscoveryPollInterval time.Duration

	// PingInterval is the interval between pings to each of our peers
	PingInterval time.Duration

	// HealthyTimeout is the time after which we will consider a peer down if we have not heard a ping from it
	// HealthyTimeout should be a moderate multiple of PingInterval (e.g. 10x)
	HealthyTimeout time.Duration

	// dnsProvider is used to register fallback DNS names found from discovery
	dnsProvider dns.Provider

	// dnsSuffix is the suffix added to the node names for discovery fallbacks
	dnsSuffix string
}

func NewServer(ctx context.Context, myInfo PeerInfo, serverTLSConfig *tls.Config, discovery discovery.Interface, defaultPort int, dnsProvider dns.Provider, dnsSuffix string, clientTLSConfig *tls.Config) (*Server, error) {
	s := &Server{
		context:         ctx,
		discovery:       discovery,
		defaultPort:     defaultPort,
		clientTLSConfig: clientTLSConfig,
		myInfo:          myInfo,
		peers:           make(map[PeerId]*peer),
		dnsProvider:     dnsProvider,
		dnsSuffix:       dnsSuffix,

		DiscoveryPollInterval: defaultDiscoveryPollInterval,
		PingInterval:          defaultPingInterval,
		HealthyTimeout:        defaultHealthyTimeout,
	}

	var opts []grpc.ServerOption
	if serverTLSConfig != nil {
		klog.Infof("starting GRPC server using TLS, ServerName=%q", serverTLSConfig.ServerName)
		opts = []grpc.ServerOption{
			grpc.Creds(credentials.NewTLS(serverTLSConfig)),
		}
	} else {
		klog.Warningf("starting insecure GRPC server")
	}

	s.grpcServer = grpc.NewServer(opts...)

	return s, nil
}

var _ ClusterServiceServer = &Server{}

func (s *Server) ListenAndServe(ctx context.Context, listen string) error {
	go s.runDiscovery(ctx)

	klog.Infof("GRPC server listening on %q", listen)

	lis, err := net.Listen("tcp", listen)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", listen, err)
	}

	go func() {
		<-ctx.Done()
		klog.Infof("context closed; forcing close of listening socket %q", listen)
		err := lis.Close()
		if err != nil {
			klog.Warningf("error closing listening socket %q: %v", listen, err)
		}
	}()

	RegisterClusterServiceServer(s.grpcServer, s)
	return s.grpcServer.Serve(lis)
}

func (s *Server) GrpcServer() *grpc.Server {
	return s.grpcServer
}

// Ping is just nodes pinging each other, part of the discovery protocol
func (s *Server) Ping(ctx context.Context, request *PingRequest) (*PingResponse, error) {
	klog.V(8).Infof("got ping %s", request)

	if request.Info == nil || request.Info.Id == "" {
		klog.Warningf("ping request did not have id: %s", request)
	} else {
		s.updateFromPingRequest(request)
	}

	response := &PingResponse{
		Info: &s.myInfo,
	}
	return response, nil
}
