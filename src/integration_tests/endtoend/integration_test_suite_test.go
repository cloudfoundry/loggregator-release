package integration_test

import (
	"testing"

	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"os/exec"
	"fmt"
	"net/http"
	"github.com/pivotal-golang/localip"
	"time"
)

func TestIntegrationTest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "End-to-end Integration Test Suite")
}

var (
	LocalIPAddress string

	etcdRunner  *etcdstorerunner.ETCDClusterRunner
	etcdAdapter storeadapter.StoreAdapter

	metronExecutablePath string
	dopplerExecutablePath string
	trafficControllerExecutablePath string

	metronSession  *gexec.Session
	dopplerSession *gexec.Session
	tcSession      *gexec.Session
)

var _ = BeforeSuite(func() {
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(49623, 1)
	etcdRunner.Start()
	etcdAdapter = etcdRunner.Adapter()

	metronExecutablePath = buildComponent("metron")
	dopplerExecutablePath = buildComponent("doppler")
	trafficControllerExecutablePath = buildComponent("trafficcontroller")

	// Wait for etcd to startup
	waitOnURL("http://localhost:49623")
})

var _ = BeforeEach(func() {
	dopplerSession = startComponent(dopplerExecutablePath, "doppler", 35, "--config=fixtures/doppler.json", "--debug")

	var err error
	LocalIPAddress, err = localip.LocalIP()
	Expect(err).ToNot(HaveOccurred())

	// Wait for doppler to startup
	Eventually(func() error {
		_, err := etcdAdapter.Get("healthstatus/doppler/z1/doppler_z1/0")
		return err
	}, 11).Should(BeNil())

	metronSession = startComponent(metronExecutablePath, "metron", 34, "--config=fixtures/metron.json", "--debug")

	// Wait for metron to startup
	waitOnURL("http://" + LocalIPAddress + ":49633")

	tcSession = startComponent(trafficControllerExecutablePath, "tc", 36, "--config=fixtures/trafficcontroller.json", "--debug", "--disableAccessControl")

	// Wait for traffic controller to startup
	waitOnURL("http://" + LocalIPAddress + ":49630")

	// Metron will report that it is up when it really isn't (at least according to the /varz endpoint)
	time.Sleep(200 * time.Millisecond)
})

func buildComponent(componentName string) (pathToComponent string){
	var err error
	pathToComponent, err = gexec.Build(componentName)
	Expect(err).ToNot(HaveOccurred())
	return pathToComponent
}

func startComponent(path string, shortName string, colorCode uint64, arg ...string) *gexec.Session {
	var session *gexec.Session
	var err error
	startCommand := exec.Command(path, arg...)
	session, err = gexec.Start(
		startCommand,
		gexec.NewPrefixedWriter(fmt.Sprintf("\x1b[32m[o]\x1b[%dm[%s]\x1b[0m ", colorCode, shortName), GinkgoWriter),
		gexec.NewPrefixedWriter(fmt.Sprintf("\x1b[91m[e]\x1b[%dm[%s]\x1b[0m ", colorCode, shortName), GinkgoWriter))
	Expect(err).ShouldNot(HaveOccurred())
	return session
}

func waitOnURL(url string) {
	Eventually(func() error {
		_, err := http.Get(url)
		return err
	}, 3).ShouldNot(HaveOccurred())
}

var _ = AfterEach(func() {
	metronSession.Kill().Wait()
	dopplerSession.Kill().Wait()
	tcSession.Kill().Wait()
})

var _ = AfterSuite(func() {
	etcdAdapter.Disconnect()
	etcdRunner.Stop()
	gexec.CleanupBuildArtifacts()
})
