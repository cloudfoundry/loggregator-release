package doppler_test

import (
	"os/exec"
	"testing"

	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
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

var (
	dopplerSession   *gexec.Session
	localIPAddress   string
	etcdPort         int
	etcdRunner       *etcdstorerunner.ETCDClusterRunner
	pathToEchoServer string
)

var _ = BeforeSuite(func() {
	etcdPort = 5555
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1)
	etcdRunner.Start()

	pathToDopplerExec, err := gexec.Build("doppler")
	Expect(err).NotTo(HaveOccurred())

	pathToEchoServer, err = gexec.Build("integration_tests/doppler/echoserver")
	Expect(err).NotTo(HaveOccurred())

	command := exec.Command(pathToDopplerExec, "--config=fixtures/doppler.json", "--debug")
	dopplerSession, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	Eventually(dopplerSession.Out).Should(gbytes.Say("Startup: doppler server started"))
	localIPAddress, _ = localip.LocalIP()

	Eventually(func() error {
		_, err := etcdRunner.Adapter().Get("healthstatus/doppler/z1/doppler_z1/0")
		return err
	}).ShouldNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	dopplerSession.Kill()
	etcdRunner.Adapter().Disconnect()
	etcdRunner.Stop()
	gexec.CleanupBuildArtifacts()
})
