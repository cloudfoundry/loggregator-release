package v2

import (
	"log"
	"net"

	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"

	"google.golang.org/grpc"
)

type Server struct {
	listener net.Listener
	addr     string
	rx       *Receiver
	opts     []grpc.ServerOption
}

func NewServer(addr string, rx *Receiver, opts ...grpc.ServerOption) *Server {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Panicf("failed to listen: %v", err)
	}
	return &Server{
		listener: listener,
		addr:     addr,
		rx:       rx,
		opts:     opts,
	}
}

func (s *Server) Addr() net.Addr {
	return s.listener.Addr()
}

func (s *Server) Start() {
	grpcServer := grpc.NewServer(s.opts...)
	v2.RegisterIngressServer(grpcServer, s.rx)

	if err := grpcServer.Serve(s.listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
