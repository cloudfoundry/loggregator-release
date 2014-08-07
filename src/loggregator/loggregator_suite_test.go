package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"fmt"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/onsi/ginkgo/config"
	loggregator "loggregator"
	"loggregator/iprange"
	"testing"
)

var (
	loggregatorInstance *loggregator.Loggregator
)

func TestLoggregator(t *testing.T) {

	RegisterFailHandler(Fail)

	etcdPort := 5500 + (config.GinkgoConfig.ParallelNode-1)*10
	etcdRunner := etcdstorerunner.NewETCDClusterRunner(etcdPort, 1)
	etcdRunner.Start()

	etcdUrl := fmt.Sprintf("http://localhost:%d", etcdPort)
	loggregatorConfig := &loggregator.Config{
		EtcdUrls:                  []string{etcdUrl},
		EtcdMaxConcurrentRequests: 10,

		Index: 0,
		LegacyIncomingMessagesPort:    3456,
		DropsondeIncomingMessagesPort: 3457,
		OutgoingPort:                  8083,
		LogFilePath:                   "",
		MaxRetainedLogMessages:        100,
		WSMessageBufferSize:           100,
		SharedSecret:                  "secret",
		SkipCertVerify:                true,
		BlackListIps:                  []iprange.IPRange{},
	}
	cfcomponent.Logger = loggertesthelper.Logger()

	loggregatorInstance = loggregator.New("127.0.0.1", loggregatorConfig, loggertesthelper.Logger())
	go loggregatorInstance.Start()

	RunSpecs(t, "Loggregator Suite")

	loggregatorInstance.Stop()
	etcdRunner.Stop()
}
