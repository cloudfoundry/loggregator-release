package testutil

import (
	"integration_tests"
	"net"
	"plumbing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type DopplerIngestorServer interface {
	plumbing.DopplerIngestorServer
}

type Server struct {
	port     int
	server   *grpc.Server
	listener net.Listener
	*mockDopplerIngestorServer
}

func NewServer() (*Server, error) {
	tlsConfig, err := plumbing.NewMutualTLSConfig(
		integration_tests.ServerCertFilePath(),
		integration_tests.ServerKeyFilePath(),
		integration_tests.CAFilePath(),
		"",
	)
	if err != nil {
		return nil, err
	}
	transportCreds := credentials.NewTLS(tlsConfig)
	mockDoppler := newMockDopplerIngestorServer()

	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, err
	}

	s := grpc.NewServer(grpc.Creds(transportCreds))
	plumbing.RegisterDopplerIngestorServer(s, mockDoppler)

	go s.Serve(lis)

	return &Server{
		port:                      lis.Addr().(*net.TCPAddr).Port,
		server:                    s,
		listener:                  lis,
		mockDopplerIngestorServer: mockDoppler,
	}, nil
}

func (s *Server) URI() string {
	return s.listener.Addr().String()
}

func (s *Server) Port() int {
	return s.port
}

func (s *Server) Stop() error {
	err := s.listener.Close()
	s.server.Stop()
	return err
}
