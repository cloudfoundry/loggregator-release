package main_test

import (
	"testing"

	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	gikgoConfig "github.com/onsi/ginkgo/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	etcdRunner *etcdstorerunner.ETCDClusterRunner
	etcdPort   int
)

func TestDoppler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Doppler Suite")
}

var _ = BeforeSuite(func() {
	etcdPort = 5500 + (gikgoConfig.GinkgoConfig.ParallelNode-1)*10
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1, nil)
	etcdRunner.Start()

	cfcomponent.Logger = loggertesthelper.Logger()
})

var _ = AfterSuite(func() {
	etcdRunner.Adapter(nil).Disconnect()
	etcdRunner.Stop()
})
