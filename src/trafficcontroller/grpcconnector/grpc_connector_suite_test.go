//go:generate hel

package grpcconnector_test

import (
	"log"
	"net"
	"plumbing"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestGRPCconnector(t *testing.T) {
	grpclog.SetLogger(log.New(GinkgoWriter, "", log.LstdFlags))
	log.SetOutput(GinkgoWriter)
	RegisterFailHandler(Fail)
	RunSpecs(t, "GRPC Connector Suite")
}

func startListener(addr string) net.Listener {
	var lis net.Listener
	f := func() error {
		var err error
		lis, err = net.Listen("tcp", addr)
		return err
	}
	Eventually(f).ShouldNot(HaveOccurred())

	return lis
}

func startGRPCServer(ds plumbing.DopplerServer, addr string) (net.Listener, *grpc.Server) {
	lis := startListener(addr)
	tlsConfig, err := plumbing.NewTLSConfig(
		"./fixtures/server.crt",
		"./fixtures/server.key",
		"./fixtures/loggregator-ca.crt",
		"doppler",
	)
	Expect(err).ToNot(HaveOccurred())
	transportCreds := credentials.NewTLS(tlsConfig)
	s := grpc.NewServer(grpc.Creds(transportCreds))
	plumbing.RegisterDopplerServer(s, ds)
	go s.Serve(lis)

	return lis, s
}
