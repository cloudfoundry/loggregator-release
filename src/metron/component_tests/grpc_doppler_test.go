package component_test

import (
	"net"
	"plumbing"

	"google.golang.org/grpc"
)

func StartFakeGRPCDoppler() (int, *mockDopplerIngestorServer, func()) {
	mockDoppler := newMockDopplerIngestorServer()

	lis, err := net.Listen("tcp4", "localhost:0")
	if err != nil {
		panic(err)
	}

	s := grpc.NewServer()
	plumbing.RegisterDopplerIngestorServer(s, mockDoppler)

	go s.Serve(lis)
	port := HomeAddrToPort(lis.Addr())

	return port, mockDoppler, func() {
		s.Stop()
		lis.Close()
	}
}
