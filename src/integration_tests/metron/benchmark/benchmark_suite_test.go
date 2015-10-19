package benchmark_test

import (
	"os/exec"

	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"

	"github.com/onsi/gomega/gexec"
)

func TestBenchmark(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Benchmark Suite")
}

var metronSession *gexec.Session
var etcdRunner *etcdstorerunner.ETCDClusterRunner
var etcdPort int

var _ = BeforeSuite(func() {
	pathToMetronExecutable, err := gexec.Build("metron")
	Expect(err).ShouldNot(HaveOccurred())

	etcdPort = 4001
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1, nil)
	etcdRunner.Start()

	command := exec.Command(pathToMetronExecutable, "--config=fixtures/metron.json")

	metronSession, err = gexec.Start(command, gexec.NewPrefixedWriter("[o][metron]", GinkgoWriter), gexec.NewPrefixedWriter("[e][metron]", GinkgoWriter))
	Expect(err).ShouldNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	metronSession.Kill().Wait()
	gexec.CleanupBuildArtifacts()

	etcdRunner.Adapter(nil).Disconnect()
	etcdRunner.Stop()
})
