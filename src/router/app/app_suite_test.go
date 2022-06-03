package app_test

import (
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/grpclog"

	"testing"
)

func TestApp(t *testing.T) {
	l := grpclog.NewLoggerV2(GinkgoWriter, GinkgoWriter, GinkgoWriter)
	grpclog.SetLoggerV2(l)

	log.SetOutput(GinkgoWriter)

	RegisterFailHandler(Fail)
	RunSpecs(t, "App Suite")
}
