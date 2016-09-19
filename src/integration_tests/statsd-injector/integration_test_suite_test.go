package integration_test

import (
	"os/exec"

	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"code.cloudfoundry.org/localip"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"

	"github.com/onsi/ginkgo/config"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

func TestIntegrationTest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "IntegrationTest Suite")
}

var metronSession *gexec.Session
var statsdInjectorSession *gexec.Session
var etcdRunner *etcdstorerunner.ETCDClusterRunner
var etcdPort int
var localIPAddress string

var pathToGoStatsdClient string

var _ = BeforeSuite(func() {
	pathToMetronExecutable, err := gexec.Build("metron")
	Expect(err).ShouldNot(HaveOccurred())

	pathToGoStatsdClient, err = gexec.Build("statsd-injector/tools/statsdGoClient")
	Expect(err).NotTo(HaveOccurred())

	command := exec.Command(pathToMetronExecutable, "--config=fixtures/metron.json", "--debug")

	metronSession, err = gexec.Start(command, gexec.NewPrefixedWriter("[o][metron]", GinkgoWriter), gexec.NewPrefixedWriter("[e][metron]", GinkgoWriter))
	Expect(err).ShouldNot(HaveOccurred())

	pathToStatsdInjectorExecutable, err := gexec.Build("statsd-injector")
	Expect(err).NotTo(HaveOccurred())
	command = exec.Command(pathToStatsdInjectorExecutable, "-statsdPort=51162", "-metronPort=51161", "-logLevel=debug")

	statsdInjectorSession, err = gexec.Start(command, gexec.NewPrefixedWriter("[o][statsdInjector]", GinkgoWriter), gexec.NewPrefixedWriter("[e][statsdInjector]", GinkgoWriter))
	Expect(err).ShouldNot(HaveOccurred())

	localIPAddress, _ = localip.LocalIP()

	etcdPort = 5800 + (config.GinkgoConfig.ParallelNode-1)*10
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1, nil)
	etcdRunner.Start()

	Eventually(metronSession, 1).Should(gbytes.Say("metron started"))
	Eventually(statsdInjectorSession).Should(gbytes.Say("Listening for statsd on host :51162"))

	// Put a key in etcd so that metron doesn't block on getting an event from
	// finder service.
	exec.Command(
		"curl", "-v", "-X", "PUT",
		"localhost:4002/v2/keys/healthstatus/doppler/z1/deployment-name/42",
		"-d", "value=127.0.0.1",
	).Run()
})

var _ = AfterSuite(func() {
	metronSession.Kill().Wait()
	statsdInjectorSession.Kill().Wait()
	gexec.CleanupBuildArtifacts()

	etcdRunner.Adapter(nil).Disconnect()
	etcdRunner.Stop()
})
