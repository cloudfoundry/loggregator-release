//go:generate hel

package grpcmanager_test

import (
	"io/ioutil"
	"log"

	"google.golang.org/grpc/grpclog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestServerhandler(t *testing.T) {
	nullLogger := log.New(ioutil.Discard, "", 0)
	grpclog.SetLogger(nullLogger)

	log.SetOutput(ioutil.Discard)
	RegisterFailHandler(Fail)
	RunSpecs(t, "gRPC Manager Suite")
}
