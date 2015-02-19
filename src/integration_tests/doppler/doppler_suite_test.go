package doppler_test

import (
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func TestDoppler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Doppler Integration Suite")
}

var command *exec.Cmd

var _ = BeforeSuite(func() {
	pathToDopplerExec, err := gexec.Build("doppler")
	Expect(err).ToNot(HaveOccurred())

	command = exec.Command(pathToDopplerExec, "--config=fixtures/doppler.json", "--debug")
	gexec.Start(command, GinkgoWriter, GinkgoWriter)

})

var _ = AfterSuite(func() {
	command.Process.Kill()

	gexec.CleanupBuildArtifacts()
})
