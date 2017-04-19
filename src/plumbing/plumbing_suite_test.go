package plumbing_test

import (
	"log"
	"metric"
	"net"
	"testing"

	"plumbing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
)

func TestPlumbing(t *testing.T) {
	grpclog.SetLogger(log.New(GinkgoWriter, "", log.LstdFlags))
	metric.Setup()
	log.SetOutput(GinkgoWriter)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Plumbing Suite")
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
	s := grpc.NewServer()
	plumbing.RegisterDopplerServer(s, ds)
	go s.Serve(lis)

	return lis, s
}
