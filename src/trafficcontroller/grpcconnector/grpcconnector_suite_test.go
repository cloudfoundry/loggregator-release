package grpcconnector_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestGrpcconnector(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Grpc Connector Suite")
}
