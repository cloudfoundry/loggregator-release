package integration_tests_test

import (
	"fmt"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

func TestIntegrationTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Logfin Suite")
}

var (
	logfinExecutablePath string
)

var _ = BeforeSuite(func() {
	logfinExecutablePath = buildComponent("tools/logfin")
})

func startComponent(path string, shortName string, colorCode uint64, arg ...string) *gexec.Session {
	var session *gexec.Session
	var err error
	startCommand := exec.Command(path, arg...)
	session, err = gexec.Start(
		startCommand,
		gexec.NewPrefixedWriter(fmt.Sprintf("\x1b[32m[o]\x1b[%dm[%s]\x1b[0m ", colorCode, shortName), GinkgoWriter),
		gexec.NewPrefixedWriter(fmt.Sprintf("\x1b[91m[e]\x1b[%dm[%s]\x1b[0m ", colorCode, shortName), GinkgoWriter))
	Expect(err).ToNot(HaveOccurred())
	return session
}

func buildComponent(componentName string) (pathToComponent string) {
	var err error
	pathToComponent, err = gexec.Build(componentName)
	Expect(err).ToNot(HaveOccurred())
	return pathToComponent
}
