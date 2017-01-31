package store_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"doppler/store"
)

var _ = Describe("AppService", func() {
	var appService store.AppService

	BeforeEach(func() {
		appService = store.AppService{AppId: "app-id", Url: "syslog://example.com:12345"}
	})

	Describe("Id", func() {
		It("should have an Id", func() {
			Expect(appService.Id()).NotTo(BeZero())
		})

		It("should have a different ID for different Urls", func() {
			otherAppService := store.AppService{AppId: "app-id", Url: "syslog://example.com:12346"}
			Expect(appService.Id()).NotTo(Equal(otherAppService.Id()))
		})

		It("should return an ID that has no / or : characters", func() {
			Expect(appService.Id()).NotTo(MatchRegexp(`/`))
			Expect(appService.Id()).NotTo(MatchRegexp(`:`))
		})
	})
})
