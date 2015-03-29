package doppler_test

import (
	"os/exec"
	"testing"

	"doppler/config"
	"time"

	"github.com/cloudfoundry/storeadapter"
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
	dopplerSession *gexec.Session
	localIPAddress string
	etcdPort       int
	etcdRunner     *etcdstorerunner.ETCDClusterRunner
	etcdAdapter    storeadapter.StoreAdapter

	pathToDopplerExec    string
	pathToHTTPEchoServer string
	pathToTCPEchoServer  string
)

var _ = BeforeSuite(func() {
	etcdPort = 5555
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1)
	etcdRunner.Start()
	etcdAdapter = etcdRunner.Adapter()

	var err error
	pathToDopplerExec, err = gexec.Build("doppler")
	Expect(err).NotTo(HaveOccurred())

	pathToHTTPEchoServer, err = gexec.Build("tools/httpechoserver")
	Expect(err).NotTo(HaveOccurred())

	pathToTCPEchoServer, err = gexec.Build("tools/tcpechoserver")
	Expect(err).NotTo(HaveOccurred())

})

var _ = BeforeEach(func() {
	var err error

	etcdRunner.Reset()

	command := exec.Command(pathToDopplerExec, "--config=fixtures/doppler.json", "--debug")
	dopplerSession, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	Eventually(dopplerSession.Out, 3).Should(gbytes.Say("Startup: doppler server started"))
	localIPAddress, _ = localip.LocalIP()
	Eventually(func() error {
		_, err := etcdAdapter.Get("healthstatus/doppler/z1/doppler_z1/0")
		return err
	}, time.Second+config.HeartbeatInterval).ShouldNot(HaveOccurred())
})

var _ = AfterEach(func() {
	dopplerSession.Kill()
	Eventually(dopplerSession).Should(gexec.Exit())
})

var _ = AfterSuite(func() {
	etcdAdapter.Disconnect()
	etcdRunner.Stop()
	gexec.CleanupBuildArtifacts()
})
