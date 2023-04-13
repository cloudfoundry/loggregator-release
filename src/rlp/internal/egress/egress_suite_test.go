package egress_test

import (
	"log"

	"google.golang.org/grpc/grpclog"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestEgress(t *testing.T) {
	l := grpclog.NewLoggerV2(GinkgoWriter, GinkgoWriter, GinkgoWriter)
	grpclog.SetLoggerV2(l)

	log.SetOutput(GinkgoWriter)

	RegisterFailHandler(Fail)
	RunSpecs(t, "Egress Suite")
}
