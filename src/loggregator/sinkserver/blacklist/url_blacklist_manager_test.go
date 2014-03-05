package blacklist_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"loggregator/iprange"
	"loggregator/sinkserver/blacklist"
	"net/url"
)

var _ = Describe("UrlBlacklistManager", func() {
	var urlBlacklistManager *blacklist.URLBlacklistManager

	BeforeEach(func() {
		urlBlacklistManager = blacklist.New([]iprange.IPRange{iprange.IPRange{Start: "14.15.16.17", End: "14.15.16.20"}})
	})

	Describe("CheckUrl", func() {
		It("returns the URL and no error if the URL is valid and not blacklisted", func() {
			outputURL, err := urlBlacklistManager.CheckUrl("http://10.10.10.10")
			url, _ := url.ParseRequestURI("http://10.10.10.10")

			Expect(outputURL).To(Equal(url))
			Expect(err).To(BeNil())
		})

		It("returns blacklist error if the URL is blacklisted", func() {
			_, err := urlBlacklistManager.CheckUrl("http://14.15.16.18")

			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("Syslog Drain URL is blacklisted"))
		})

		It("returns incomplete URL error if the URL is invalid", func() {
			_, err := urlBlacklistManager.CheckUrl("http://")

			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(MatchRegexp("(?i:incomplete url)"))
		})
	})
})
