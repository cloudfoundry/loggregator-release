package v2_test

import (
	"errors"
	"sync"

	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/fakestoreadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"doppler/store/v2"
)

var _ = Describe("AppServiceStoreWatcherUnit", func() {
	Context("when there is an error", func() {
		var adapter *fakeStoreAdapter

		BeforeEach(func() {
			adapter = newFSA()
			watcher, _, _ := v2.NewAppServiceStoreWatcher(adapter, v2.NewAppServiceCache())

			go watcher.Run()
		})

		It("calls watch again", func() {
			Eventually(adapter.GetWatchCounter).Should(Equal(1))
			adapter.WatchErrChannel <- errors.New("Haha")
			Eventually(adapter.GetWatchCounter).Should(Equal(2))
		})
	})
})

type fakeStoreAdapter struct {
	*fakestoreadapter.FakeStoreAdapter
	watchCounter int
	sync.Mutex
}

func (fsa *fakeStoreAdapter) Watch(key string) (events <-chan storeadapter.WatchEvent, stop chan<- bool, errors <-chan error) {
	events, _, errors = fsa.FakeStoreAdapter.Watch(key)

	fsa.Lock()
	defer fsa.Unlock()
	fsa.watchCounter++

	return events, make(chan bool), errors
}

func (fsa *fakeStoreAdapter) GetWatchCounter() int {
	fsa.Lock()
	defer fsa.Unlock()

	return fsa.watchCounter
}

func newFSA() *fakeStoreAdapter {
	return &fakeStoreAdapter{
		FakeStoreAdapter: fakestoreadapter.New(),
	}
}
