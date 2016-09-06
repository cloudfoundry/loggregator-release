package lats_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	latsConfig "lats/config"
	"lats/helpers"
	"os/exec"
	"testing"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var config *latsConfig.TestConfig

func TestLats(t *testing.T) {
	RegisterFailHandler(Fail)

	var metronSession *gexec.Session
	config = latsConfig.Load()

	BeforeSuite(func() {
		config.SaveMetronConfig()
		helpers.Initialize(config)
		metronSession = setupMetron()
	})

	AfterSuite(func() {
		metronSession.Kill().Wait()
	})

	RunSpecs(t, "Lats Suite")
}

func setupMetron() *gexec.Session {
	pathToMetronExecutable, err := gexec.Build("metron")
	Expect(err).ShouldNot(HaveOccurred())

	command := exec.Command(pathToMetronExecutable, "--config=fixtures/metron.json", "--debug")
	metronSession, err := gexec.Start(command, gexec.NewPrefixedWriter("[o][metron]", GinkgoWriter), gexec.NewPrefixedWriter("[e][metron]", GinkgoWriter))
	Expect(err).ShouldNot(HaveOccurred())

	Eventually(metronSession.Buffer).Should(gbytes.Say("Chose protocol"))
	Consistently(metronSession.Exited).ShouldNot(BeClosed())

	return metronSession
}
