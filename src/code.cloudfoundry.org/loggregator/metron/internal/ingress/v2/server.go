package v2

import (
	"log"
	"net"

	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"

	"google.golang.org/grpc"
)

type Server struct {
	addr string
	rx   *Receiver
	opts []grpc.ServerOption
}

func NewServer(addr string, rx *Receiver, opts ...grpc.ServerOption) *Server {
	return &Server{
		addr: addr,
		rx:   rx,
		opts: opts,
	}
}

func (s *Server) Start() {
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer(s.opts...)
	v2.RegisterIngressServer(grpcServer, s.rx)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
