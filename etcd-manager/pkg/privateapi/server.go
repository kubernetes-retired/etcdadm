package privateapi

import (
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"kope.io/etcd-manager/pkg/privateapi/discovery"
)

type Server struct {
	myInfo     PeerInfo
	grpcServer *grpc.Server

	clientTLSConfig *tls.Config

	discovery discovery.Interface

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
}

func NewServer(ctx context.Context, myInfo PeerInfo, serverTLSConfig *tls.Config, discovery discovery.Interface, clientTLSConfig *tls.Config) (*Server, error) {
	s := &Server{
		discovery:       discovery,
		clientTLSConfig: clientTLSConfig,
		myInfo:          myInfo,
		peers:           make(map[PeerId]*peer),
		context:         ctx,

		DiscoveryPollInterval: defaultDiscoveryPollInterval,
		PingInterval:          defaultPingInterval,
		HealthyTimeout:        defaultHealthyTimeout,
	}

	var opts []grpc.ServerOption
	if serverTLSConfig != nil {
		glog.Infof("starting GRPC server using TLS, ServerName=%q", serverTLSConfig.ServerName)
		opts = []grpc.ServerOption{
			grpc.Creds(credentials.NewTLS(serverTLSConfig)),
		}
	} else {
		glog.Warningf("starting insecure GRPC server")
	}

	s.grpcServer = grpc.NewServer(opts...)

	return s, nil
}

var _ ClusterServiceServer = &Server{}

func (s *Server) ListenAndServe(ctx context.Context, listen string) error {
	go s.runDiscovery(ctx)

	glog.Infof("GRPC server listening on %q", listen)

	lis, err := net.Listen("tcp", listen)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", listen, err)
	}

	go func() {
		<-ctx.Done()
		glog.Infof("context closed; forcing close of listening socket %q", listen)
		err := lis.Close()
		if err != nil {
			glog.Warningf("error closing listening socket %q: %v", listen, err)
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
	glog.V(8).Infof("got ping %s", request)

	if request.Info == nil || request.Info.Id == "" {
		glog.Warningf("ping request did not have id: %s", request)
	} else {
		s.updateFromPingRequest(request)
	}

	response := &PingResponse{
		Info: &s.myInfo,
	}
	return response, nil
}
