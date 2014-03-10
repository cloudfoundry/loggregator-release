package store_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"loggregator/domain"
	. "loggregator/store"
	"loggregator/store/cache"
)

var _ = Describe("AppServiceStoreIntegration", func() {
	var (
		incomingChan  chan domain.AppServices
		outAddChan    <-chan domain.AppService
		outRemoveChan <-chan domain.AppService
	)

	BeforeEach(func() {
		adapter := etcdRunner.Adapter()

		incomingChan = make(chan domain.AppServices)
		c := cache.NewAppServiceCache()
		var watcher *AppServiceStoreWatcher
		watcher, outAddChan, outRemoveChan = NewAppServiceStoreWatcher(adapter, c)
		go watcher.Run()

		store := NewAppServiceStore(adapter, watcher)
		go store.Run(incomingChan)
	})

	It("should receive, store, and republish AppServices", func(done Done) {
		appServices := domain.AppServices{AppId: "12345", Urls: []string{"syslog://foo"}}
		incomingChan <- appServices

		Expect(<-outAddChan).To(Equal(domain.AppService{
			AppId: "12345", Url: "syslog://foo",
		}))

		appServices = domain.AppServices{AppId: "12345", Urls: []string{"syslog://foo", "syslog://bar"}}
		incomingChan <- appServices

		Expect(<-outAddChan).To(Equal(domain.AppService{
			AppId: "12345", Url: "syslog://bar",
		}))

		appServices = domain.AppServices{AppId: "12345", Urls: []string{"syslog://bar"}}

		incomingChan <- appServices

		Expect(<-outRemoveChan).To(Equal(domain.AppService{
			AppId: "12345", Url: "syslog://foo",
		}))
		close(done)
	}, 5)
})
