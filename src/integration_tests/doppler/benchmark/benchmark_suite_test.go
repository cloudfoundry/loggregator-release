package benchmark_test

import (
	"os/exec"
	"testing"

	"doppler/config"
	"time"

	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/pivotal-golang/localip"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

func TestBenchmark(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Benchmark Suite")
}

var (
	dopplerSession *gexec.Session
	localIPAddress string
	etcdRunner     *etcdstorerunner.ETCDClusterRunner
	etcdAdapter    storeadapter.StoreAdapter

	pathToDopplerExec string
)

var _ = BeforeSuite(func() {
	etcdPort := 5555
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1, nil)
	etcdRunner.Start()
	etcdAdapter = etcdRunner.Adapter(nil)

	var err error
	pathToDopplerExec, err = gexec.Build("doppler")
	Expect(err).NotTo(HaveOccurred())
})

var _ = BeforeEach(func() {
	var err error

	etcdRunner.Reset()

	command := exec.Command(pathToDopplerExec, "--config=fixtures/doppler.json")
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
	dopplerSession.Kill().Wait()
})

var _ = AfterSuite(func() {
	etcdAdapter.Disconnect()
	etcdRunner.Stop()
	gexec.CleanupBuildArtifacts()
})
