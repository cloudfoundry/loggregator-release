package main_test

import (
	"log"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/grpclog"

	"testing"
)

func TestRlp(t *testing.T) {
	l := grpclog.NewLoggerV2(GinkgoWriter, GinkgoWriter, GinkgoWriter)
	grpclog.SetLoggerV2(l)

	log.SetOutput(GinkgoWriter)

	RegisterFailHandler(Fail)
	RunSpecs(t, "RLP compile main")
}
