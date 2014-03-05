package loggregator_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"fmt"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/onsi/ginkgo/config"
	"loggregator"
	"loggregator/iprange"
	"testing"
)

func TestLoggregator(t *testing.T) {

	RegisterFailHandler(Fail)

	etcdPort := 5000 + (config.GinkgoConfig.ParallelNode-1)*10
	etcdRunner := etcdstorerunner.NewETCDClusterRunner(etcdPort, 1)
	etcdRunner.Start()

	etcdUrl := fmt.Sprintf("http://localhost:%d", etcdPort)
	loggregatorConfig := &loggregator.Config{
		EtcdUrls:                  []string{etcdUrl},
		EtcdMaxConcurrentRequests: 10,

		Index:                  0,
		IncomingPort:           3456,
		OutgoingPort:           8083,
		LogFilePath:            "",
		MaxRetainedLogMessages: 100,
		WSMessageBufferSize:    100,
		SharedSecret:           "secret",
		SkipCertVerify:         true,
		BlackListIps:           []iprange.IPRange{},
	}
	cfcomponent.Logger = loggertesthelper.Logger()

	l := loggregator.New("127.0.0.1", loggregatorConfig, loggertesthelper.Logger())
	l.Start()

	RunSpecs(t, "Loggregator Suite")
	l.Stop()
	etcdRunner.Stop()
}
