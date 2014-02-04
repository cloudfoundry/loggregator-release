package store_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"loggregator/domain"
	. "loggregator/store"
)

var _ = PDescribe("AppServiceStoreIntegration", func() {
	var (
		incomingChan  chan domain.AppServices
		outAddChan    <-chan domain.AppService
		outRemoveChan <-chan domain.AppService
	)

	BeforeEach(func() {
		adapter := etcdRunner.Adapter()

		incomingChan = make(chan domain.AppServices)
		store := NewAppServiceStore(adapter, incomingChan)
		go store.Run()

		var watcher *AppServiceStoreWatcher
		watcher, outAddChan, outRemoveChan = NewAppServiceStoreWatcher(adapter)
		go watcher.Run()
		ensureWatchersAreHookedUp()
	})

	It("should receive, store, and republish AppServices", func() {
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
	})
})
