package sinks_test

import (
	"log"
	"testing"

	"google.golang.org/grpc/grpclog"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSinks(t *testing.T) {
	l := grpclog.NewLoggerV2(GinkgoWriter, GinkgoWriter, GinkgoWriter)
	grpclog.SetLoggerV2(l)

	log.SetOutput(GinkgoWriter)

	RegisterFailHandler(Fail)
	RunSpecs(t, "Sinks Suite")
}
