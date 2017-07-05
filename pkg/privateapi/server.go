package privateapi

import (
	"fmt"
	"net"
	"sync"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type Server struct {
	myInfo     PeerInfo
	grpcServer *grpc.Server

	discovery Discovery

	mutex sync.Mutex
	peers map[PeerId]*peer
}

func NewServer(myInfo PeerInfo, discovery Discovery) (*Server, error) {
	s := &Server{
		discovery: discovery,
		myInfo:    myInfo,
		peers:     make(map[PeerId]*peer),
	}

	var opts []grpc.ServerOption
	//if *tls {
	//	creds, err := credentials.NewServerTLSFromFile(*certFile, *keyFile)
	//	if err != nil {
	//		grpclog.Fatalf("Failed to generate credentials %v", err)
	//	}
	//	opts = []grpc.ServerOption{grpc.Creds(creds)}
	//}
	s.grpcServer = grpc.NewServer(opts...)

	return s, nil
}

var _ ClusterServiceServer = &Server{}

func (s *Server) ListenAndServe(listen string) error {
	go s.runDiscovery()

	lis, err := net.Listen("tcp", listen)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", listen, err)
	}

	RegisterClusterServiceServer(s.grpcServer, s)
	return s.grpcServer.Serve(lis)
}

func (s *Server) GrpcServer() *grpc.Server {
	return s.grpcServer
}

// Ping just pings another node, part of the discovery protocol
func (s *Server) Ping(ctx context.Context, request *PingRequest) (*PingResponse, error) {
	glog.Infof("Got ping %s", request)

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
