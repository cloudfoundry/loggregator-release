package integration_test

import (
	"integration_tests/runners"
	"io/ioutil"
	"os"

	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/pivotal-golang/localip"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"

	"github.com/onsi/ginkgo/config"
	"github.com/onsi/gomega/gexec"
)

func TestIntegrationTest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "IntegrationTest Suite")
}

var tmpdir string
var port int
var metronRunner *runners.MetronRunner
var etcdRunner *etcdstorerunner.ETCDClusterRunner
var etcdAdapter storeadapter.StoreAdapter
var localIPAddress string

var _ = SynchronizedBeforeSuite(func() []byte {
	metronPath, err := gexec.Build("metron", "-race")
	Expect(err).ShouldNot(HaveOccurred())
	return []byte(metronPath)
}, func(path []byte) {
	metronPath := string(path)

	var err error
	tmpdir, err = ioutil.TempDir("", "metronintg")
	Expect(err).NotTo(HaveOccurred())

	etcdPort := 5800 + (config.GinkgoConfig.ParallelNode)*10
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1, nil)
	etcdRunner.Start()
	etcdAdapter = etcdRunner.Adapter(nil)

	localIPAddress, err = localip.LocalIP()
	Expect(err).NotTo(HaveOccurred())

	port = 51000 + config.GinkgoConfig.ParallelNode*10
	metronRunner = &runners.MetronRunner{
		Path:          metronPath,
		TempDir:       tmpdir,
		LegacyPort:    port,
		MetronPort:    port + 1,
		DropsondePort: 3457 + config.GinkgoConfig.ParallelNode*10,
		EtcdRunner:    etcdRunner,

		CertFile: "../fixtures/client.crt",
		KeyFile:  "../fixtures/client.key",
		CAFile:   "../fixtures/loggregator-ca.crt",
	}
})

var _ = SynchronizedAfterSuite(func() {
	if etcdRunner != nil {
		etcdRunner.Stop()
	}
	os.RemoveAll(tmpdir)
}, func() {
	gexec.CleanupBuildArtifacts()
})

var _ = BeforeEach(func() {
	etcdRunner.Reset()
})
