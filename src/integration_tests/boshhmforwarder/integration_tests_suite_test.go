package integration_tests_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"os/exec"
	"testing"

	"github.com/onsi/gomega/gexec"
)

var (
	session *gexec.Session
)

func TestIntegrationTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bosh HM Forwarder - Integration Tests Suite")
}

var _ = BeforeSuite(func() {
	boshForwarderExecutable, err := gexec.Build("boshhmforwarder", "-race")
	Expect(err).ToNot(HaveOccurred())

	command := exec.Command(boshForwarderExecutable, "--configPath", "./integration_config.json")

	session, err = gexec.Start(command, gexec.NewPrefixedWriter("[o][boshhmforwarder]", GinkgoWriter), gexec.NewPrefixedWriter("[e][boshhmforwarder]", GinkgoWriter))
	Expect(err).ToNot(HaveOccurred())
	Consistently(session.Exited).ShouldNot(BeClosed())
})

var _ = AfterSuite(func() {
	if session != nil {
		session.Kill().Wait()
	}

	gexec.CleanupBuildArtifacts()
})
