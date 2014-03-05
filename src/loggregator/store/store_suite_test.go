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
	"reflect"
	"testing"
	"time"
)

var etcdRunner *etcdstorerunner.ETCDClusterRunner

func TestStore(t *testing.T) {
	cfcomponent.Logger = loggertesthelper.Logger()
	registerSignalHandler()

	RegisterFailHandler(Fail)

	etcdPort := 5000 + (config.GinkgoConfig.ParallelNode-1)*10
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1)
	etcdRunner.Start()

	RunSpecs(t, "Store Suite")

	etcdRunner.Stop()
}

var _ = BeforeEach(func() {
	etcdRunner.Stop()
	etcdRunner.Start()
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

func assertNoDataOnChannel(channel interface{}) {
	Consistently(channel).ShouldNot(Receive())
	channelValue := reflect.ValueOf(channel)
	timeout := reflect.ValueOf(time.After(2 * time.Millisecond))

	winnerIndex, _, _ := reflect.Select([]reflect.SelectCase{
		reflect.SelectCase{Dir: reflect.SelectRecv, Chan: channelValue},
		reflect.SelectCase{Dir: reflect.SelectRecv, Chan: timeout},
	})

	if winnerIndex == 0 {
		Fail("Should not have any data on the channel", 1)
	}
}

func ensureWatchersAreHookedUp() {
	time.Sleep(100 * time.Millisecond) //give watchers time to get running
}
