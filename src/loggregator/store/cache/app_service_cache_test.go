package cache_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "loggregator/store/cache"

	"loggregator/domain"
)

var _ = Describe("AppServiceCache", func() {
	var appServiceCache AppServiceCache
	var appService1 domain.AppService
	var appService2 domain.AppService

	BeforeEach(func() {
		appServiceCache = NewAppServiceCache()
		appService1 = domain.AppService{AppId: "12345", Url: "http://example.com"}
		appService2 = domain.AppService{AppId: "12345", Url: "http://example.com:1234"}

		appServiceCache.Add(appService1)
		appServiceCache.Add(appService2)
	})

	Describe("Get", func() {
		It("returns the AppServices for the given AppId", func() {
			appServices := appServiceCache.Get(appService1.AppId)

			Expect(len(appServices)).To(Equal(2))
			Expect(appServices[0]).To(Equal(appService1))
			Expect(appServices[1]).To(Equal(appService2))
		})

		It("returns an empty slice of AppServices for an unknown AppId", func() {
			appServices := appServiceCache.Get("non-existant-app-id")

			Expect(len(appServices)).To(Equal(0))
		})
	})

	Describe("Size", func() {
		It("returns the total number of AppServices for all AppIds", func() {
			anotherAppService := domain.AppService{AppId: "98765", Url: "http://foo.com"}
			appServiceCache.Add(anotherAppService)

			Expect(appServiceCache.Size()).To(Equal(3))
		})
	})

	Describe("Add", func() {
		It("does not add the given AppService to the cache twice", func() {
			Expect(appServiceCache.Size()).To(Equal(2))

			appServiceCache.Add(appService1)
			Expect(appServiceCache.Size()).To(Equal(2))
		})
	})

	Describe("Remove", func() {
		It("removes the given AppService from the cache", func() {
			Expect(appServiceCache.Size()).To(Equal(2))

			appServiceCache.Remove(appService1)
			Expect(appServiceCache.Size()).To(Equal(1))
		})

		It("removes all the AppServices for a given app", func() {
			Expect(appServiceCache.Size()).To(Equal(2))

			appServiceCache.Remove(appService1)
			appServiceCache.Remove(appService2)
			Expect(appServiceCache.Size()).To(Equal(0))
		})
	})

	Describe("RemoveApp", func() {
		It("removes the AppServices for the given AppId from the cache", func() {
			Expect(appServiceCache.Size()).To(Equal(2))

			appServiceCache.RemoveApp(appService1.AppId)
			Expect(appServiceCache.Size()).To(Equal(0))
		})

		It("returns the removed AppServices", func() {
			appServices := appServiceCache.RemoveApp(appService1.AppId)
			Expect(len(appServices)).To(Equal(2))
			Expect(appServices[0]).To(Equal(appService1))
			Expect(appServices[1]).To(Equal(appService2))
		})
	})

	Describe("Exists", func() {
		It("returns true for known AppService", func() {
			Expect(appServiceCache.Exists(appService1)).To(Equal(true))
		})

		It("returns the removed AppServices", func() {
			anotherAppService := AppService{AppId: "98765", Url: "http://foo.com"}
			Expect(appServiceCache.Exists(anotherAppService)).To(Equal(false))
		})
	})
})
