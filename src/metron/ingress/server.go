package ingress

import (
	"log"
	"net"

	v2 "plumbing/v2"

	"google.golang.org/grpc"
)

type Server struct {
	addr string
	rx   *Receiver
}

func NewServer(addr string, rx *Receiver) *Server {
	return &Server{
		addr: addr,
		rx:   rx,
	}
}

func (s *Server) Start() {
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	v2.RegisterMetronIngressServer(grpcServer, s.rx)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
