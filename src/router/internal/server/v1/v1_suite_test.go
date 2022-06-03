//go:generate hel

package v1_test

import (
	"log"

	"google.golang.org/grpc/grpclog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestServerhandler(t *testing.T) {
	l := grpclog.NewLoggerV2(GinkgoWriter, GinkgoWriter, GinkgoWriter)
	grpclog.SetLoggerV2(l)

	log.SetOutput(GinkgoWriter)

	RegisterFailHandler(Fail)
	RunSpecs(t, "Doppler Server v1 Suite")
}
