package v2_test

import (
	"doppler/store/v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppService", func() {
	var appService v2.ServiceInfo

	BeforeEach(func() {
		appService = v2.NewServiceInfo("app-id", "syslog://example.com:12345", "org.space.app.1")
	})

	Describe("Id", func() {
		It("should have an Id", func() {
			Expect(appService.Id()).NotTo(BeZero())
		})

		It("should have a different ID for different Urls", func() {
			otherAppService := v2.NewServiceInfo("app-id", "syslog://example.com:12346", "org.space.app.1")
			Expect(appService.Id()).NotTo(Equal(otherAppService.Id()))
		})

		It("should return an ID that has no / or : characters", func() {
			Expect(appService.Id()).NotTo(MatchRegexp(`/`))
			Expect(appService.Id()).NotTo(MatchRegexp(`:`))
		})
	})
})
