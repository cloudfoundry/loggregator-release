package app_test

import (
	"log"
	"metric"

	"google.golang.org/grpc/grpclog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestApp(t *testing.T) {
	log.SetOutput(GinkgoWriter)
	grpclog.SetLogger(log.New(GinkgoWriter, "", 0))
	metric.Setup()
	RegisterFailHandler(Fail)
	RunSpecs(t, "RLP Suite")
}
