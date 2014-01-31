package store_test

import (
	"github.com/cloudfoundry/storeadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "loggregator/store"
	"path"

	"loggregator/domain"
)

var _ = Describe("AppServiceStore", func() {
	var store *AppServiceStore
	var adapter storeadapter.StoreAdapter
	var incomingChan chan domain.AppServices

	var app1Service1 domain.AppService
	var app1Service2 domain.AppService
	var app2Service1 domain.AppService

	assertInStore := func(appServices ...domain.AppService) {
		for _, appService := range appServices {
			Eventually(func() error {
				_, err := adapter.Get(path.Join("/loggregator/services/", appService.AppId, appService.Id()))
				return err
			}).ShouldNot(HaveOccurred())
		}
	}

	assertNotInStore := func(appServices ...domain.AppService) {
		for _, appService := range appServices {
			Eventually(func() error {
				_, err := adapter.Get(path.Join("/loggregator/services/", appService.AppId, appService.Id()))
				return err
			}).Should(Equal(storeadapter.ErrorKeyNotFound))
		}
	}

	assertAppNotInStore := func(appIds ...string) {
		for _, appId := range appIds {
			Eventually(func() error {
				_, err := adapter.Get(path.Join("/loggregator/services/", appId))
				return err
			}).Should(Equal(storeadapter.ErrorKeyNotFound))
		}
	}

	BeforeEach(func() {
		adapter = etcdRunner.Adapter()
		incomingChan = make(chan domain.AppServices)

		store = NewAppServiceStore(adapter, incomingChan)

		app1Service1 = domain.AppService{AppId: "app-1", Url: "syslog://example.com:12345"}
		app1Service2 = domain.AppService{AppId: "app-1", Url: "syslog://example.com:12346"}
		app2Service1 = domain.AppService{AppId: "app-2", Url: "syslog://example.com:12345"}
	})

	AfterEach(func() {
		err := adapter.Disconnect()
		Expect(err).NotTo(HaveOccurred())
		if incomingChan != nil {
			close(incomingChan)
		}
	})

	Context("when the incoming chan is closed", func() {
		BeforeEach(func() {
			close(incomingChan)
		})

		AfterEach(func() {
			incomingChan = nil //want the clean-up in AfterEach not to panic
		})

		It("should return", func(done Done) {
			store.Run()
			close(done)
		})
	})

	Context("when the store has data", func() {
		BeforeEach(func() {
			adapter.Create(buildNode(app1Service1))
			adapter.Create(buildNode(app1Service2))
			adapter.Create(buildNode(app2Service1))

			go store.Run()
		})

		It("does not modify the store, if the incoming data is already there", func(done Done) {
			events, stop, _ := adapter.Watch("/loggregator/services")

			incomingChan <- domain.AppServices{
				AppId: app1Service1.AppId,
				Urls:  []string{app1Service1.Url, app1Service2.Url},
			}

			assertInStore(app1Service1, app1Service2, app2Service1)

			assertNoDataOnChannel(events)

			stop <- true

			close(done)
		})

		Context("when there is new data for the store", func() {
			Context("when an existing app has a new service", func() {
				It("adds that service to the store", func(done Done) {
					app2Service2 := domain.AppService{app2Service1.AppId, "syslog://new.example.com:12345"}

					incomingChan <- domain.AppServices{
						AppId: app2Service1.AppId,
						Urls:  []string{app2Service1.Url, app2Service2.Url},
					}

					assertInStore(app2Service1, app2Service2)

					close(done)
				})
			})

			Context("when a new app appears", func() {
				It("adds that app and its services to the store", func(done Done) {
					app3Service1 := domain.AppService{"app-3", "syslog://app3.example.com:12345"}
					app3Service2 := domain.AppService{"app-3", "syslog://app3.example.com:12346"}

					incomingChan <- domain.AppServices{
						AppId: app3Service1.AppId,
						Urls:  []string{app3Service1.Url, app3Service2.Url},
					}

					assertInStore(app3Service1, app3Service2)

					close(done)
				})
			})
		})

		Context("when a service or app should be removed", func() {
			Context("when an existing app loses one of its services", func() {
				It("removes that service from the store", func(done Done) {
					incomingChan <- domain.AppServices{
						AppId: app1Service1.AppId,
						Urls:  []string{app1Service1.Url},
					}

					assertInStore(app1Service1)
					assertNotInStore(app1Service2)

					close(done)
				})
			})

			Context("when an existing app loses all of its services", func() {
				It("removes the app entirely", func(done Done) {
					incomingChan <- domain.AppServices{
						AppId: app1Service1.AppId,
						Urls:  []string{},
					}

					assertNotInStore(app1Service1, app1Service2)
					assertAppNotInStore(app1Service1.AppId)

					incomingChan <- domain.AppServices{
						AppId: app1Service1.AppId,
						Urls:  []string{app1Service1.Url, app1Service2.Url},
					}

					assertInStore(app1Service1, app1Service2)

					close(done)
				})
			})

			Context("when an app is deleted (i.e. we get no reports about it on the incoming channel)", func() {
				PIt("(eventually) removes the app entirely", func() {})
			})
		})

		Describe("with multiple updates to the same app-id", func() {
			It("should perform the updates correctly in the store", func(done Done) {
				incomingChan <- domain.AppServices{
					AppId: app1Service1.AppId,
					Urls:  []string{app1Service1.Url},
				}

				assertInStore(app1Service1)
				assertNotInStore(app1Service2)

				incomingChan <- domain.AppServices{
					AppId: app1Service1.AppId,
					Urls:  []string{app1Service1.Url, app1Service2.Url},
				}

				assertInStore(app1Service1, app1Service2)

				incomingChan <- domain.AppServices{
					AppId: app1Service1.AppId,
					Urls:  []string{app1Service2.Url},
				}

				assertInStore(app1Service2)
				assertNotInStore(app1Service1)

				close(done)
			})
		})
	})
})
