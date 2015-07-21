package endtoend_test

import (
	"testing"

	"fmt"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-golang/localip"
	"net/http"
	"os/exec"
	"time"
	"runtime"
)

func TestIntegrationTest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "End-to-end Integration Test Suite")
}

var (
	LocalIPAddress string

	etcdRunner  *etcdstorerunner.ETCDClusterRunner
	etcdAdapter storeadapter.StoreAdapter

	metronExecutablePath            string
	dopplerExecutablePath           string
	trafficControllerExecutablePath string

	metronSession  *gexec.Session
	dopplerSession *gexec.Session
	tcSession      *gexec.Session
)

var _ = BeforeSuite(func() {
	runtime.GOMAXPROCS(runtime.NumCPU())
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
	const (
		BLUE       = 34
		PURPLE     = 35
		LIGHT_BLUE = 36
	)

	dopplerSession = startComponent(dopplerExecutablePath, "doppler", PURPLE, "--config=fixtures/doppler.json")
	var err error
	LocalIPAddress, err = localip.LocalIP()
	Expect(err).ToNot(HaveOccurred())

	// Wait for doppler to startup
	Eventually(func() error {
		_, err := etcdAdapter.Get("healthstatus/doppler/z1/doppler_z1/0")
		return err
	}, 11).Should(BeNil())

	metronSession = startComponent(metronExecutablePath, "metron", BLUE, "--config=fixtures/metron.json")

	// Wait for metron to startup
	waitOnURL("http://" + LocalIPAddress + ":49633")

	tcSession = startComponent(trafficControllerExecutablePath, "tc", LIGHT_BLUE, "--config=fixtures/trafficcontroller.json", "--disableAccessControl")

	// Wait for traffic controller to startup
	waitOnURL("http://" + LocalIPAddress + ":49630")

	// Metron will report that it is up when it really isn't (at least according to the /varz endpoint)
	time.Sleep(200 * time.Millisecond)
})

func buildComponent(componentName string) (pathToComponent string) {
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
