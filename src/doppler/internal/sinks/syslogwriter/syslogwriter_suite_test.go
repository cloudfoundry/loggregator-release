package syslogwriter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

func TestSyslogwriter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SyslogWriter Suite")
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
