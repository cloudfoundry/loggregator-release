package v1_test

import (
	"doppler/store/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppService", func() {
	var appService v1.ServiceInfo

	BeforeEach(func() {
		appService = v1.NewServiceInfo("app-id", "syslog://example.com:12345")
	})

	Describe("Id", func() {
		It("should have an Id", func() {
			Expect(appService.Id()).NotTo(BeZero())
		})

		It("should have a different ID for different Urls", func() {
			otherAppService := v1.NewServiceInfo("app-id", "syslog://example.com:12346")
			Expect(appService.Id()).NotTo(Equal(otherAppService.Id()))
		})

		It("should return an ID that has no / or : characters", func() {
			Expect(appService.Id()).NotTo(MatchRegexp(`/`))
			Expect(appService.Id()).NotTo(MatchRegexp(`:`))
		})
	})
})
