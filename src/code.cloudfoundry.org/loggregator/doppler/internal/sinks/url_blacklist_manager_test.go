package sinks_test

import (
	"net/url"

	"code.cloudfoundry.org/loggregator/doppler/internal/sinks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("UrlBlacklistManager", func() {
	var urlBlacklistManager *sinks.URLBlacklistManager

	BeforeEach(func() {
		urlBlacklistManager = sinks.NewBlackListManager([]sinks.IPRange{{Start: "14.15.16.17", End: "14.15.16.20"}})
	})

	Describe("CheckUrl", func() {
		It("returns the URL and no error if the URL is valid and not blacklisted", func() {
			outputURL, err := urlBlacklistManager.CheckUrl("http://10.10.10.10")
			Expect(err).NotTo(HaveOccurred())
			url, err := url.ParseRequestURI("http://10.10.10.10")
			Expect(err).NotTo(HaveOccurred())

			Expect(outputURL).To(Equal(url))
		})

		It("returns the URL and no error if the domain can't be resolved", func() {
			outputURL, err := urlBlacklistManager.CheckUrl("http://some.invalid.host")
			Expect(err).NotTo(HaveOccurred())
			url, err := url.ParseRequestURI("http://some.invalid.host")
			Expect(err).NotTo(HaveOccurred())

			Expect(outputURL).To(Equal(url))
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
