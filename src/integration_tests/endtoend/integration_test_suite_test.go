package endtoend_test

import (
	"doppler/dopplerservice"
	"testing"

	"fmt"
	"net/http"
	"os/exec"
	"runtime"

	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-golang/localip"
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

	dopplerConfig string
)

var _ = BeforeSuite(func() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(49623, 1, nil)
	etcdRunner.Start()
	etcdAdapter = etcdRunner.Adapter(nil)
	metronExecutablePath = buildComponent("metron")
	dopplerExecutablePath = buildComponent("doppler")
	trafficControllerExecutablePath = buildComponent("trafficcontroller")

	// Wait for etcd to startup
	waitOnURL("http://127.0.0.1:49623")
})

var _ = BeforeEach(func() {
	etcdRunner.Reset()

	dopplerConfig = "dopplerudp"
})

var _ = JustBeforeEach(func() {
	const (
		BLUE       = 34
		PURPLE     = 35
		LIGHT_BLUE = 36
	)

	dopplerSession = startComponent(dopplerExecutablePath, "doppler", PURPLE, "--config=fixtures/"+dopplerConfig+".json")
	var err error
	LocalIPAddress, err = localip.LocalIP()
	Expect(err).ToNot(HaveOccurred())

	metronSession = startComponent(metronExecutablePath, "metron", BLUE, "--config=fixtures/metron.json")

	tcSession = startComponent(trafficControllerExecutablePath, "tc", LIGHT_BLUE, "--config=fixtures/trafficcontroller.json", "--disableAccessControl")

	// Wait for traffic controller to startup
	waitOnURL("http://" + LocalIPAddress + ":49630")

	// Wait for metron
	Eventually(metronSession.Buffer).Should(gbytes.Say("metron started"))

	// wait for doppler to register
	key := fmt.Sprintf("%s/z1/doppler_z1/0", dopplerservice.LEGACY_ROOT)
	Eventually(func() bool {
		_, err := etcdAdapter.Get(key)
		return err == nil
	}, 1).Should(BeTrue())
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
