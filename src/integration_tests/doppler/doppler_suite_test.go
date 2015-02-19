package doppler_test

import (
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-golang/localip"
)

func TestDoppler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Doppler Integration Suite")
}

var session *gexec.Session
var localIPAddress string

var _ = BeforeSuite(func() {
	pathToDopplerExec, err := gexec.Build("doppler")
	Expect(err).NotTo(HaveOccurred())

	command := exec.Command(pathToDopplerExec, "--config=fixtures/doppler.json", "--debug")
	session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	Eventually(session.Out).Should(gbytes.Say("Setting up the doppler server"))

	localIPAddress, _ = localip.LocalIP()
})

var _ = AfterSuite(func() {
	session.Kill()

	gexec.CleanupBuildArtifacts()
})
