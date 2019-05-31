package main_test

import (
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/grpclog"

	"testing"
)

func TestRlp(t *testing.T) {
	log.SetOutput(GinkgoWriter)
	grpclog.SetLogger(log.New(GinkgoWriter, "", 0))
	RegisterFailHandler(Fail)
	RunSpecs(t, "RLP compile main")
}
