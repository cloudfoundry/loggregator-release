package sinks_test

import (
	"io/ioutil"
	"log"
	"testing"

	"google.golang.org/grpc/grpclog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestSinks(t *testing.T) {
	log.SetOutput(GinkgoWriter)
	grpclog.SetLogger(log.New(ioutil.Discard, "", 0))
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sinks Suite")
}
