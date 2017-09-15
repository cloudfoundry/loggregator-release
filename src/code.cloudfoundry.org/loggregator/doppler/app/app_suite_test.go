package app_test

import (
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/grpclog"

	"testing"
)

func TestApp(t *testing.T) {
	log.SetOutput(GinkgoWriter)
	grpclog.SetLogger(log.New(GinkgoWriter, "", 0))
	RegisterFailHandler(Fail)
	RunSpecs(t, "App Suite")
}
