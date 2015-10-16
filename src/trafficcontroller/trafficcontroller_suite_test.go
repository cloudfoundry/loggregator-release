package main_test

import (
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"

	"testing"
)

var etcdRunner *etcdstorerunner.ETCDClusterRunner
var etcdPort int

var _ = BeforeSuite(func() {
	etcdPort = 5800 + (config.GinkgoConfig.ParallelNode-1)*10
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1, nil)
	etcdRunner.Start()
})

var _ = AfterSuite(func() {
	etcdRunner.Adapter(nil).Disconnect()
	etcdRunner.Stop()
})

func TestTrafficcontroller(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Trafficcontroller Suite")

}

var _ = BeforeEach(func() {
	adapter := etcdRunner.Adapter(nil)
	adapter.Disconnect()
	etcdRunner.Reset()
	adapter.Connect()
})
