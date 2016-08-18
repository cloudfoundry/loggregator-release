package debugtools_test

import (
	"testing"

	"fmt"
	"net/http"
	"os/exec"

	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func TestDebugTools(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "End-to-end Integration Test Suite")
}

var (
	etcdRunner  *etcdstorerunner.ETCDClusterRunner
	etcdAdapter storeadapter.StoreAdapter

	metronExecutablePath            string
	dopplerExecutablePath           string
	trafficControllerExecutablePath string
)

var _ = BeforeSuite(func() {
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(49623, 1, nil)
	etcdRunner.Start()
	etcdAdapter = etcdRunner.Adapter(nil)
	metronExecutablePath = buildComponent("metron")
	dopplerExecutablePath = buildComponent("doppler")
	trafficControllerExecutablePath = buildComponent("trafficcontroller")

	// Wait for etcd to startup
	Eventually(func() bool { return checkEndpoint("49623", "v2/keys", http.StatusOK) }, 5).Should(Equal(true))
})

var _ = BeforeEach(func() {
	etcdRunner.Reset()
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

func checkEndpoint(port, endpoint string, status int) bool {
	resp, _ := http.Get("http://localhost:" + port + "/" + endpoint)
	if resp != nil {
		return resp.StatusCode == status
	}

	return false
}

var _ = AfterSuite(func() {
	etcdAdapter.Disconnect()
	etcdRunner.Stop()
	gexec.CleanupBuildArtifacts()
})
