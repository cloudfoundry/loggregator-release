package store_test

import (
	"code.cloudfoundry.org/loggregator/doppler/internal/store"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppServiceCache", func() {
	var appServiceCache store.AppServiceWatcherCache
	var app1Service1, app1Service2, app2Service1 store.ServiceInfo

	BeforeEach(func() {
		appServiceCache = store.NewAppServiceCache()

		app1Service1 = store.NewServiceInfo("app-1", "syslog://example.com:12345", "org.space.app.1")
		app1Service2 = store.NewServiceInfo("app-1", "syslog://example.com:12346", "org.space.app.1")
		app2Service1 = store.NewServiceInfo("app-2", "syslog://example.com:12345", "org.space.app.1")

		appServiceCache.Add(app1Service1)
		appServiceCache.Add(app1Service2)
		appServiceCache.Add(app2Service1)
	})

	Describe("Get", func() {
		It("returns the AppServices for the given AppId", func() {
			appServices := appServiceCache.Get(app1Service1.AppId())

			Expect(len(appServices)).To(Equal(2))
			Expect(appServices).To(ContainElement(app1Service1))
			Expect(appServices).To(ContainElement(app1Service2))
		})

		It("returns an empty slice of AppServices for an unknown AppId", func() {
			appServices := appServiceCache.Get("non-existent-app-id")

			Expect(len(appServices)).To(Equal(0))
		})
	})

	Describe("Size", func() {
		It("returns the total number of AppServices for all AppIds", func() {
			anotherAppService := store.NewServiceInfo("98765", "http://foo.com", "org.space.app.1")
			appServiceCache.Add(anotherAppService)

			Expect(appServiceCache.Size()).To(Equal(4))
		})
	})

	Describe("Add", func() {
		It("does not add the given AppService to the cache twice", func() {
			Expect(appServiceCache.Size()).To(Equal(3))

			appServiceCache.Add(app1Service1)
			Expect(appServiceCache.Size()).To(Equal(3))
		})
	})

	Describe("Remove", func() {
		It("removes the given AppService from the cache", func() {
			Expect(appServiceCache.Size()).To(Equal(3))

			appServiceCache.Remove(app1Service1)
			Expect(appServiceCache.Size()).To(Equal(2))
		})

		It("removes all the AppServices for a given app", func() {
			Expect(appServiceCache.Size()).To(Equal(3))

			appServiceCache.Remove(app1Service1)
			appServiceCache.Remove(app1Service2)
			Expect(appServiceCache.Size()).To(Equal(1))
		})
	})

	Describe("RemoveApp", func() {
		It("removes the AppServices for the given AppId from the cache", func() {
			Expect(appServiceCache.Size()).To(Equal(3))

			appServiceCache.RemoveApp(app1Service1.AppId())
			Expect(appServiceCache.Size()).To(Equal(1))
		})

		It("returns the removed AppServices", func() {
			appServices := appServiceCache.RemoveApp(app1Service1.AppId())
			Expect(len(appServices)).To(Equal(2))
			Expect(appServices).To(ContainElement(app1Service1))
			Expect(appServices).To(ContainElement(app1Service2))
		})
	})

	Describe("Exists", func() {
		It("returns true for known AppService", func() {
			Expect(appServiceCache.Exists(app1Service1)).To(BeTrue())
		})

		It("returns the removed AppServices", func() {
			anotherAppService := store.NewServiceInfo("98765", "http://foo.com", "org.space.app.1")
			Expect(appServiceCache.Exists(anotherAppService)).To(BeFalse())
		})
	})
})
