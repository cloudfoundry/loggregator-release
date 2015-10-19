package dopplerservice_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"doppler/config"
	"testing"

	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	ginkgoConfig "github.com/onsi/ginkgo/config"
	"github.com/pivotal-golang/localip"
)

func TestAnnouncer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DopplerService Suite")
}

var (
	localIP     string
	conf        config.Config
	etcdRunner  *etcdstorerunner.ETCDClusterRunner
	etcdAdapter storeadapter.StoreAdapter
)

var _ = BeforeSuite(func() {
	localIP, _ = localip.LocalIP()

	etcdPort := 5500 + ginkgoConfig.GinkgoConfig.ParallelNode*10
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1, nil)
	etcdRunner.Start()

	etcdAdapter = etcdRunner.Adapter(nil)

	conf = config.Config{
		JobName: "doppler_z1",
		Index:   0,
		EtcdMaxConcurrentRequests: 10,
		EtcdUrls:                  etcdRunner.NodeURLS(),
		Zone:                      "z1",
		DropsondeIncomingMessagesPort: 1234,
	}
})

var _ = BeforeEach(func() {
	etcdRunner.Reset()
})

var _ = AfterSuite(func() {
	etcdAdapter.Disconnect()
	etcdRunner.Stop()
})
