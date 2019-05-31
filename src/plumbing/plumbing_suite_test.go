package plumbing_test

import (
	"io/ioutil"
	"log"
	"net"
	"testing"

	"code.cloudfoundry.org/loggregator/plumbing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestPlumbing(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	grpclog.SetLogger(log.New(ioutil.Discard, "", log.LstdFlags))
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
