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
	metronSession *gexec.Session
	etcdRunner    *etcdstorerunner.ETCDClusterRunner
	etcdPort      int
)

var _ = BeforeSuite(func() {
	etcdPort = 4001
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1, nil)
	etcdRunner.Start()
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()

	etcdRunner.Adapter(nil).Disconnect()
	etcdRunner.Stop()
})
