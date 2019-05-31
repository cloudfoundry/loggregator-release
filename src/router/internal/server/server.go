package server

import (
	"fmt"
	"log"
	"net"
	"sync"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	plumbingv1 "code.cloudfoundry.org/loggregator/plumbing"
	"google.golang.org/grpc"
)

// Server represents the gRPC interface of Doppler. It wraps a gRPC server and
// handles service registration.
type Server struct {
	listener   net.Listener
	grpcServer *grpc.Server

	mu      sync.Mutex
	stopped bool
}

// NewServer is the constructor for Server. The constructor attempts to open a
// listener on the provided port and will return an error if that binding
// fails.
func NewServer(
	port uint16,
	v1Ingress plumbingv1.DopplerIngestorServer,
	v1Egress plumbingv1.DopplerServer,
	v2Ingress loggregator_v2.IngressServer,
	v2Egress loggregator_v2.EgressServer,
	srvOpts ...grpc.ServerOption,
) (*Server, error) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Printf("Failed to start listener (port=%d) for gRPC: %s", port, err)
		return nil, err
	}
	log.Printf("grpc bound to: %s", lis.Addr())

	grpcServer := grpc.NewServer(srvOpts...)
	plumbingv1.RegisterDopplerIngestorServer(grpcServer, v1Ingress)
	plumbingv1.RegisterDopplerServer(grpcServer, v1Egress)
	loggregator_v2.RegisterIngressServer(grpcServer, v2Ingress)
	loggregator_v2.RegisterEgressServer(grpcServer, v2Egress)

	s := &Server{
		listener:   lis,
		grpcServer: grpcServer,
	}

	return s, nil
}

// Start initiates the gRPC server.
func (g *Server) Start() {
	log.Printf("Starting gRPC server on %s", g.listener.Addr().String())
	if err := g.grpcServer.Serve(g.listener); err != nil {
		g.mu.Lock()
		stopped := g.stopped
		g.mu.Unlock()

		if !stopped {
			log.Fatalf("Failed to start gRPC server: %s", err)
		}
	}
}

// Stop closes the gRPC server.
func (g *Server) Stop() {
	g.mu.Lock()
	g.stopped = true
	g.mu.Unlock()
	g.grpcServer.Stop()
}

// Addr provides the address of the listener.
func (g *Server) Addr() string {
	return g.listener.Addr().String()
}
