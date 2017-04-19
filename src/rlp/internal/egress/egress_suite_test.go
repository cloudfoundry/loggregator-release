package egress_test

//go:generate hel

import (
	"log"
	"metric"

	"google.golang.org/grpc/grpclog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestEgress(t *testing.T) {
	log.SetOutput(GinkgoWriter)
	grpclog.SetLogger(log.New(GinkgoWriter, "", 0))
	metric.Setup()

	RegisterFailHandler(Fail)
	RunSpecs(t, "Egress Suite")
}
