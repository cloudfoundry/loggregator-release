package grpcconnector_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestGRPCconnector(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GRPC Connector Suite")
}
