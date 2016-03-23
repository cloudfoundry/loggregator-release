package metronbenchmark_test

import (
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var (
	pathToMetronExecutable    string
	pathToMetronBenchmarkExec string
	metronSession             *gexec.Session
	etcdRunner                *etcdstorerunner.ETCDClusterRunner
	etcdPort                  int
)

func TestMetronbenchmark(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metronbenchmark Suite")
}

var _ = BeforeSuite(func() {
	etcdPort = 4001
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1, nil)
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
