package main_test

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"

	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/cloudfoundry/yagnats"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"

	"testing"
)

var etcdRunner *etcdstorerunner.ETCDClusterRunner
var etcdPort int

var _ = BeforeSuite(func() {
	etcdPort = 5800 + (config.GinkgoConfig.ParallelNode-1)*10
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1)
	etcdRunner.Start()
})

var _ = AfterSuite(func() {
	etcdRunner.Adapter().Disconnect()
	etcdRunner.Stop()
})

func TestTrafficcontroller(t *testing.T) {
	cfcomponent.DefaultYagnatsClientProvider = func(logger *gosteno.Logger, c *cfcomponent.Config) (yagnats.NATSClient, error) {
		return nil, nil
	}
	RegisterFailHandler(Fail)

	RunSpecs(t, "Trafficcontroller Suite")

}

var _ = BeforeEach(func() {
	etcdRunner.Adapter().Disconnect()
	etcdRunner.Reset()
	etcdRunner.Adapter().Connect()
})
