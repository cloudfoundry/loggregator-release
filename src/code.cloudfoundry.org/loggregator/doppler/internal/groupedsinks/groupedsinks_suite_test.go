package groupedsinks_test

import (
	"io/ioutil"
	"log"
	"testing"

	"google.golang.org/grpc/grpclog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestGroupedsinks(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	grpclog.SetLogger(log.New(ioutil.Discard, "", 0))
	RegisterFailHandler(Fail)
	RunSpecs(t, "GroupedSinks Suite")
}
