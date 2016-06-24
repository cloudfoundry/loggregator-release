package metron_test

import (
	"fmt"
	"integration_tests/runners"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-golang/localip"
)

func TestIntegrationTest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metron Integration Suite")
}

var (
	tmpdir         string
	port           int
	metronRunner   *runners.MetronRunner
	etcdRunner     *etcdstorerunner.ETCDClusterRunner
	etcdAdapter    storeadapter.StoreAdapter
	localIPAddress string

	etcdPath    string
	dopplerPath string
	metronPath  string
)

var _ = SynchronizedBeforeSuite(func() []byte {
	// TODO: rip this out
	metronPathOld, err := gexec.Build("metron", "-race")
	Expect(err).ShouldNot(HaveOccurred())
	return []byte(metronPathOld)
}, func(path []byte) {
	metronPathOld := string(path)

	var err error
	tmpdir, err = ioutil.TempDir("", "metronintg")
	Expect(err).NotTo(HaveOccurred())

	etcdPort := 5800 + (config.GinkgoConfig.ParallelNode)*10
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1, nil)
	etcdRunner.Start()
	etcdAdapter = etcdRunner.Adapter(nil)

	localIPAddress, err = localip.LocalIP()
	Expect(err).NotTo(HaveOccurred())

	waitOnURL(fmt.Sprintf("http://127.0.0.1:%d", etcdPort))
	port = 51000 + config.GinkgoConfig.ParallelNode*10
	metronRunner = &runners.MetronRunner{
		Path:          metronPathOld,
		TempDir:       tmpdir,
		LegacyPort:    port,
		MetronPort:    port + 1,
		DropsondePort: 3457 + config.GinkgoConfig.ParallelNode*10,
		EtcdRunner:    etcdRunner,

		CertFile: "../fixtures/client.crt",
		KeyFile:  "../fixtures/client.key",
		CAFile:   "../fixtures/loggregator-ca.crt",
	}

	{
		var err error

		rand.Seed(time.Now().UnixNano())

		metronPath, err = gexec.Build("metron", "-race")
		Expect(err).ToNot(HaveOccurred())

		dopplerPath, err = gexec.Build("doppler", "-race")
		Expect(err).ToNot(HaveOccurred())

		etcdPath, err = gexec.Build("github.com/coreos/etcd", "-race")
		Expect(err).ToNot(HaveOccurred())
	}
})

var _ = SynchronizedAfterSuite(func() {
	if etcdRunner != nil {
		etcdRunner.Stop()
	}
	os.RemoveAll(tmpdir)
}, func() {
	gexec.CleanupBuildArtifacts()

	{
		os.Remove(etcdPath)
		os.Remove(dopplerPath)
		os.Remove(metronPath)
	}
})

var _ = BeforeEach(func() {
	etcdRunner.Reset()
})

func waitOnURL(url string) {
	Eventually(func() error {
		_, err := http.Get(url)
		return err
	}, 3).ShouldNot(HaveOccurred())
}
