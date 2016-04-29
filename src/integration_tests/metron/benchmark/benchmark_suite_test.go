package benchmark_test

import (
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

var (
	pathToMetronBenchmarkExec string
	pathToMetronExecutable    string
	metronSession             *gexec.Session
	etcdRunner                *etcdstorerunner.ETCDClusterRunner
)

var _ = BeforeSuite(func() {
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(4001, 1, nil)
	etcdRunner.Start()

	var err error
	pathToMetronExecutable, err = gexec.Build("metron")
	Expect(err).ToNot(HaveOccurred())
	pathToMetronBenchmarkExec, err = gexec.Build("tools/metronbenchmark")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()

	etcdRunner.Adapter(nil).Disconnect()
	etcdRunner.Stop()
})
