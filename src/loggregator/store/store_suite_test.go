package store_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/onsi/ginkgo/config"
	"loggregator/domain"
	"os"
	"os/signal"
	"path"
	"testing"
	"time"
)

var etcdRunner *etcdstorerunner.ETCDClusterRunner

func TestStore(t *testing.T) {
	SetDefaultEventuallyTimeout(5 * time.Second)
	cfcomponent.Logger = loggertesthelper.Logger()
	registerSignalHandler()

	RegisterFailHandler(Fail)

	etcdPort := 5800 + (config.GinkgoConfig.ParallelNode-1)*10
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1)
	etcdRunner.Start()
	RunSpecs(t, "Store Suite")
	etcdRunner.Adapter().Disconnect()
	etcdRunner.Stop()
}

var _ = BeforeEach(func() {
	etcdRunner.Adapter().Disconnect()
	etcdRunner.Reset()
	etcdRunner.Adapter().Connect()
})

func registerSignalHandler() {
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, os.Kill)

		select {
		case <-c:
			etcdRunner.Stop()
			os.Exit(0)
		}
	}()
}

func buildNode(appService domain.AppService) storeadapter.StoreNode {
	return storeadapter.StoreNode{
		Key:   path.Join("/loggregator/services", appService.AppId, appService.Id()),
		Value: []byte(appService.Url),
	}
}
