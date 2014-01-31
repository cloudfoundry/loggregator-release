package store_test

import (
	"github.com/cloudfoundry/storeadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "loggregator/store"
	"path"

	"loggregator/domain"
)

var _ = Describe("AppServiceStoreWatcher", func() {
	var listener *AppServiceStoreWatcher
	var adapter storeadapter.StoreAdapter
	var outAddChan <-chan domain.AppService
	var outRemoveChan <-chan domain.AppService

	var app1Service1 domain.AppService
	var app1Service2 domain.AppService
	var app2Service1 domain.AppService

	drainOutgoingChannel := func(c <-chan domain.AppService, count int) []domain.AppService {
		appServices := []domain.AppService{}
		for i := 0; i < count; i++ {
			appService, ok := <-c
			if !ok {
				break
			}
			appServices = append(appServices, appService)
		}
		return appServices
	}

	BeforeEach(func() {
		adapter = etcdRunner.Adapter()

		listener, outAddChan, outRemoveChan = NewAppServiceStoreWatcher(adapter)

		app1Service1 = domain.AppService{AppId: "app-1", Url: "syslog://example.com:12345"}
		app1Service2 = domain.AppService{AppId: "app-1", Url: "syslog://example.com:12346"}
		app2Service1 = domain.AppService{AppId: "app-2", Url: "syslog://example.com:12345"}
	})

	AfterEach(func() {
		err := adapter.Disconnect()
		Expect(err).NotTo(HaveOccurred())
	})

	PDescribe("Shutdown", func() {
		BeforeEach(func() {
			go listener.Run()

			adapter.Disconnect()
		})

		It("should close the outgoing channels", func() {
			Expect(outAddChan).To(BeClosed())
			Expect(outRemoveChan).To(BeClosed())
		})
	})

	Describe("Loading listener state on startup", func() {
		Context("when the store is empty", func() {
			It("should not send anything on the output channels", func() {
				go listener.Run()

				assertNoDataOnChannel(outAddChan)
				assertNoDataOnChannel(outRemoveChan)
			})
		})

		Context("when the store has AppServices in it", func() {
			BeforeEach(func() {
				adapter.Create(buildNode(app1Service1))
				adapter.Create(buildNode(app1Service2))
				adapter.Create(buildNode(app2Service1))
			})

			It("should send all the AppServices on the output add channel", func(done Done) {
				go listener.Run()

				appServices := drainOutgoingChannel(outAddChan, 3)

				Expect(appServices).To(ContainElement(app1Service1))
				Expect(appServices).To(ContainElement(app1Service2))
				Expect(appServices).To(ContainElement(app2Service1))

				assertNoDataOnChannel(outRemoveChan)

				close(done)
			})
		})
	})

	Describe("when the store has data and listener is bootstrapped", func() {
		BeforeEach(func() {
			adapter.Create(buildNode(app1Service1))
			adapter.Create(buildNode(app1Service2))
			adapter.Create(buildNode(app2Service1))

			go listener.Run()
			drainOutgoingChannel(outAddChan, 3)
			ensureWatchersAreHookedUp()
		})

		It("does not send updates when the data has already been processed", func(done Done) {
			adapter.Create(buildNode(app1Service1))
			adapter.Create(buildNode(app1Service2))

			assertNoDataOnChannel(outAddChan)
			assertNoDataOnChannel(outRemoveChan)

			close(done)
		})

		Context("when there is new data in the store", func() {
			Context("when an existing app has a new service (create)", func() {
				It("adds that service to the outgoing add channel", func(done Done) {
					app2Service2 := domain.AppService{app2Service1.AppId, "syslog://new.example.com:12345"}
					adapter.Create(buildNode(app2Service2))

					Expect(<-outAddChan).To(Equal(app2Service2))
					assertNoDataOnChannel(outRemoveChan)

					close(done)
				})
			})

			Context("When an existing app gets a new service (update)", func() {
				It("adds that service to the outgoing add channel", func(done Done) {
					app2Service2 := domain.AppService{app2Service1.AppId, "syslog://new.example.com:12345"}
					adapter.SetMulti([]storeadapter.StoreNode{buildNode(app2Service2)})

					Expect(<-outAddChan).To(Equal(app2Service2))
					assertNoDataOnChannel(outRemoveChan)

					close(done)
				})
			})

			Context("when a new app appears", func() {
				It("adds that app and its services to the outgoing add channel", func(done Done) {
					app3Service1 := domain.AppService{"app-3", "syslog://app3.example.com:12345"}
					app3Service2 := domain.AppService{"app-3", "syslog://app3.example.com:12346"}

					adapter.Create(buildNode(app3Service1))
					adapter.Create(buildNode(app3Service2))

					appServices := drainOutgoingChannel(outAddChan, 2)

					Expect(appServices).To(ContainElement(app3Service1))
					Expect(appServices).To(ContainElement(app3Service2))

					assertNoDataOnChannel(outRemoveChan)

					close(done)
				})
			})
		})

		Context("When an existing service is updated", func() {
			It("should not notify the channel again", func() {
				adapter.SetMulti([]storeadapter.StoreNode{buildNode(app2Service1)})
				assertNoDataOnChannel(outAddChan)
				assertNoDataOnChannel(outRemoveChan)
			})
		})

		Context("when a service or app should be removed", func() {
			Context("when an existing app loses one of its services", func() {
				It("sends that service on the output remove channel", func(done Done) {
					adapter.Delete(path.Join("/loggregator/services", app1Service2.AppId, app1Service2.Id()))

					Expect(<-outRemoveChan).To(Equal(app1Service2))
					assertNoDataOnChannel(outAddChan)

					close(done)
				})
			})

			Context("when an existing app loses all of its services", func() {
				It("sends all of the app services on the outgoing remove channel", func(done Done) {
					adapter.Delete(path.Join("/loggregator/services", app1Service2.AppId))

					appServices := drainOutgoingChannel(outRemoveChan, 2)

					Expect(appServices).To(ContainElement(app1Service1))
					Expect(appServices).To(ContainElement(app1Service2))
					assertNoDataOnChannel(outAddChan)

					adapter.Create(buildNode(app1Service1))
					adapter.Create(buildNode(app1Service2))

					appServices = drainOutgoingChannel(outAddChan, 2)

					Expect(appServices).To(ContainElement(app1Service1))
					Expect(appServices).To(ContainElement(app1Service2))
					assertNoDataOnChannel(outRemoveChan)

					close(done)
				})
			})
		})

		Describe("with multiple updates to the same app-id", func() {
			It("should perform the updates correctly on the outgoing channels", func(done Done) {
				adapter.Delete(path.Join("/loggregator/services", app1Service2.AppId, app1Service2.Id()))

				Expect(<-outRemoveChan).To(Equal(app1Service2))
				assertNoDataOnChannel(outAddChan)

				adapter.Create(buildNode(app1Service2))

				Expect(<-outAddChan).To(Equal(app1Service2))
				assertNoDataOnChannel(outRemoveChan)

				adapter.Delete(path.Join("/loggregator/services", app1Service1.AppId, app1Service1.Id()))

				assertNoDataOnChannel(outAddChan)
				Expect(<-outRemoveChan).To(Equal(app1Service1))

				close(done)
			})
		})
	})
})
