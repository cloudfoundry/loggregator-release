package integration_tests_test

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"testing"

	"boshhmforwarder/config"
	"boshhmforwarder/logging"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var session *gexec.Session

func TestIntegrationTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bosh HM Forwarder - Integration Tests Suite")
}

var _ = BeforeSuite(func() {
	boshForwarderExecutable, err := gexec.Build("boshhmforwarder", "-race")
	Expect(err).ToNot(HaveOccurred())

	createConfig()
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
	err := os.Remove("integration_config.json")
	Expect(err).ToNot(HaveOccurred())
})

func createConfig() {
	config := &config.Config{
		IncomingPort: testPort(),
		InfoPort:     testPort(),
		DebugPort:    -1,
		MetronPort:   testPort(),
		LogLevel:     logging.DEBUG,
	}

	j, err := json.Marshal(config)
	Expect(err).ToNot(HaveOccurred())

	err = ioutil.WriteFile("integration_config.json", j, 0644)
	Expect(err).ToNot(HaveOccurred())
}

func testPort() int {
	add, _ := net.ResolveTCPAddr("tcp", ":0")
	l, _ := net.ListenTCP("tcp", add)
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port
	return port
}
