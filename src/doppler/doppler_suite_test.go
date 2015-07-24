package main_test

import (
	"doppler/config"
	"doppler/iprange"
	"fmt"
	"testing"

	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	gikgoConfig "github.com/onsi/ginkgo/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	dopplerConfig *config.Config
	etcdRunner    *etcdstorerunner.ETCDClusterRunner
	etcdPort      int
)

func TestDoppler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Doppler Suite")
}

var _ = BeforeSuite(func() {
	etcdPort = 5500 + (gikgoConfig.GinkgoConfig.ParallelNode-1)*10
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1)
	etcdRunner.Start()

	etcdUrl := fmt.Sprintf("http://localhost:%d", etcdPort)
	dopplerConfig = &config.Config{
		EtcdUrls:                  []string{etcdUrl},
		EtcdMaxConcurrentRequests: 10,

		Index: 0,
		DropsondeIncomingMessagesPort: 3457,
		OutgoingPort:                  8083,
		LogFilePath:                   "",
		MaxRetainedLogMessages:        100,
		MessageDrainBufferSize:        100,
		SharedSecret:                  "secret",
		SkipCertVerify:                true,
		BlackListIps:                  []iprange.IPRange{},
		ContainerMetricTTLSeconds:     120,
		SinkInactivityTimeoutSeconds:  2,
		UnmarshallerCount:             1,
	}
	cfcomponent.Logger = loggertesthelper.Logger()
})

var _ = AfterSuite(func() {
	etcdRunner.Adapter().Disconnect()
	etcdRunner.Stop()
})
