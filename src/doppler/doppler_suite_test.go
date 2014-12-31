package main_test

import (
	doppler "doppler"
	"doppler/iprange"
	"fmt"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/onsi/ginkgo/config"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	dopplerInstance *doppler.Doppler
	etcdRunner      *etcdstorerunner.ETCDClusterRunner
	etcdPort        int
)

func TestDoppler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Doppler Suite")
}

var _ = BeforeSuite(func() {
	etcdPort = 5500 + (config.GinkgoConfig.ParallelNode-1)*10
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1)
	etcdRunner.Start()

	etcdUrl := fmt.Sprintf("http://localhost:%d", etcdPort)
	dopplerConfig := &doppler.Config{
		EtcdUrls:                  []string{etcdUrl},
		EtcdMaxConcurrentRequests: 10,

		Index: 0,
		DropsondeIncomingMessagesPort: 3457,
		OutgoingPort:                  8083,
		LogFilePath:                   "",
		MaxRetainedLogMessages:        100,
		WSMessageBufferSize:           100,
		SharedSecret:                  "secret",
		SkipCertVerify:                true,
		BlackListIps:                  []iprange.IPRange{},
		ContainerMetricTTLSeconds:     120,
	}
	cfcomponent.Logger = loggertesthelper.Logger()

	dopplerInstance = doppler.New("127.0.0.1", dopplerConfig, loggertesthelper.Logger(), "dropsondeOrigin")
	go dopplerInstance.Start()
})

var _ = AfterSuite(func() {
	dopplerInstance.Stop()
	etcdRunner.Adapter().Disconnect()
	etcdRunner.Stop()
})
