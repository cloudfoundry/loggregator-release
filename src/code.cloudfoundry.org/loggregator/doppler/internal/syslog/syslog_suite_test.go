package syslog_test

import (
	"log"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func TestSyslog(t *testing.T) {
	log.SetOutput(GinkgoWriter)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Syslog Suite")
}

var pathToTCPEchoServer string

var _ = BeforeSuite(func() {
	var err error
	pathToTCPEchoServer, err = gexec.Build("tools/echo/cmd/tcp_server")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})
