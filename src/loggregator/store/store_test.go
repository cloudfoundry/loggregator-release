package store_test

import (
	"github.com/cloudfoundry/storeadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "loggregator/store"
	"path"
	"time"
)

func buildNode(appService AppService) storeadapter.StoreNode {
	return storeadapter.StoreNode{
		Key:   path.Join("/loggregator/services", appService.AppId, appService.Id()),
		Value: []byte(appService.Url),
	}
}

var _ = Describe("Store", func() {
	var store *Store
	var adapter storeadapter.StoreAdapter
	var outChan chan AppService
	var incomingChan chan AppServices

	var app1Service1 AppService
	var app1Service2 AppService
	var app2Service1 AppService

	assertNoOutgoingData := func() {
		select {
		case <-outChan:
			Fail("Should not have any data on the channel", 1)
		case <-time.After(2 * time.Millisecond):
			// OK
		}
	}

	drainOutgoingChannel := func(count int) []AppService {
		appServices := []AppService{}
		for i := 0; i < count; i++ {
			appServices = append(appServices, <-outChan)
		}
		return appServices
	}

	assertInStore := func(appServices ...AppService) {
		for _, appService := range appServices {
			_, err := adapter.Get(path.Join("/loggregator/services/", appService.AppId, appService.Id()))
			Expect(err).NotTo(HaveOccurred())
		}
	}

	assertNotInStore := func(appServices ...AppService) {
		for _, appService := range appServices {
			_, err := adapter.Get(path.Join("/loggregator/services/", appService.AppId, appService.Id()))
			Expect(err).To(Equal(storeadapter.ErrorKeyNotFound))
		}
	}

	assertAppNotInStore := func(appIds ...string) {
		for _, appId := range appIds {
			_, err := adapter.Get(path.Join("/loggregator/services/", appId))
			Expect(err).To(Equal(storeadapter.ErrorKeyNotFound))
		}
	}

	BeforeEach(func() {
		adapter = etcdRunner.Adapter()
		outChan = make(chan AppService)
		incomingChan = make(chan AppServices)

		store = NewStore(adapter, outChan, incomingChan)

		app1Service1 = AppService{AppId: "app-1", Url: "syslog://example.com:12345"}
		app1Service2 = AppService{AppId: "app-1", Url: "syslog://example.com:12346"}
		app2Service1 = AppService{AppId: "app-2", Url: "syslog://example.com:12345"}
	})

	Describe("Loading store state on startup", func() {
		Context("when the store is empty", func() {
			It("should not send anything on the channel", func() {
				go store.Run()

				assertNoOutgoingData()
			})
		})

		Context("when the store has AppServices in it", func() {
			BeforeEach(func() {
				adapter.Create(buildNode(app1Service1))
				adapter.Create(buildNode(app1Service2))
				adapter.Create(buildNode(app2Service1))
			})

			It("should send all the AppServices on the channel", func(done Done) {
				go store.Run()

				appServices := drainOutgoingChannel(3)

				Expect(appServices).To(ContainElement(app1Service1))
				Expect(appServices).To(ContainElement(app1Service2))
				Expect(appServices).To(ContainElement(app2Service1))

				close(done)
			})
		})
	})

	Describe("incoming channel", func() {
		BeforeEach(func() {
			adapter.Create(buildNode(app1Service1))
			adapter.Create(buildNode(app1Service2))
			adapter.Create(buildNode(app2Service1))

			go store.Run()
			drainOutgoingChannel(3) //perhaps, ideally, we don't need to do this?
		})

		Context("when it's closed", func() {
			PIt("should stop sending notifications about changes", func() {})
		})

		It("does not modify the store, if the incoming data is already there", func(done Done) {
			incomingChan <- AppServices{
				AppId: app1Service1.AppId,
				Urls:  []string{app1Service1.Url, app1Service2.Url},
			}

			assertNoOutgoingData()

			assertInStore(app1Service1, app1Service2, app2Service1)

			close(done)
		})

		Context("when there is new data for the store", func() {
			Context("when an existing app has a new service", func() {
				It("adds that service to the store", func(done Done) {
					app2Service2 := AppService{app2Service1.AppId, "syslog://new.example.com:12345"}

					incomingChan <- AppServices{
						AppId: app2Service1.AppId,
						Urls:  []string{app2Service1.Url, app2Service2.Url},
					}

					Expect(<-outChan).To(Equal(app2Service2))

					assertInStore(app2Service1, app2Service2)

					close(done)
				})
			})

			Context("when a new app appears", func() {
				It("adds that app and its services to the store", func(done Done) {
					app3Service1 := AppService{"app-3", "syslog://app3.example.com:12345"}
					app3Service2 := AppService{"app-3", "syslog://app3.example.com:12346"}

					incomingChan <- AppServices{
						AppId: app3Service1.AppId,
						Urls:  []string{app3Service1.Url, app3Service2.Url},
					}

					appServices := drainOutgoingChannel(2)

					Expect(appServices).To(ContainElement(app3Service1))
					Expect(appServices).To(ContainElement(app3Service2))

					assertInStore(app3Service1, app3Service2)

					close(done)
				})
			})
		})

		Context("when a service or app should be removed", func() {
			Context("when an existing app loses one of its services", func() {
				It("removes that service from the store", func(done Done) {
					incomingChan <- AppServices{
						AppId: app1Service1.AppId,
						Urls:  []string{app1Service1.Url},
					}

					assertNoOutgoingData()

					assertInStore(app1Service1)
					assertNotInStore(app1Service2)

					close(done)
				})
			})

			Context("when an existing app loses all of its services", func() {
				It("removes the app entirely", func(done Done) {
					incomingChan <- AppServices{
						AppId: app1Service1.AppId,
						Urls:  []string{},
					}

					assertNoOutgoingData()

					assertNotInStore(app1Service1, app1Service2)
					assertAppNotInStore(app1Service1.AppId)
					close(done)
				})
			})

			Context("when an app is deleted (i.e. we get no reports about it on the incoming channel)", func() {
				PIt("(eventually) removes the app entirely", func() {})
			})
		})

		Describe("with multiple updates to the same app-id", func() {
			It("should perform the updates correctly on the outgoing channel, and in the store", func(done Done) {
				incomingChan <- AppServices{
					AppId: app1Service1.AppId,
					Urls:  []string{app1Service1.Url},
				}

				assertNoOutgoingData()

				assertInStore(app1Service1)
				assertNotInStore(app1Service2)

				incomingChan <- AppServices{
					AppId: app1Service1.AppId,
					Urls:  []string{app1Service1.Url, app1Service2.Url},
				}

				Expect(<-outChan).To(Equal(app1Service2))

				assertInStore(app1Service1, app1Service2)

				incomingChan <- AppServices{
					AppId: app1Service1.AppId,
					Urls:  []string{app1Service2.Url},
				}

				assertNoOutgoingData()

				assertInStore(app1Service2)
				assertNotInStore(app1Service1)

				close(done)
			})
		})
	})
})
