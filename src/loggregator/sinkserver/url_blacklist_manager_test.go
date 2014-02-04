package sinkserver_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"loggregator/sinkserver"
)

var _ = Describe("UrlBlacklistManager", func() {
	var urlBlacklistManager sinkserver.URLBlacklistManager

	BeforeEach(func() {
		urlBlacklistManager = sinkserver.URLBlacklistManager{}
		urlBlacklistManager.BlacklistUrl("http://www.example.com/bad")
	})

	Describe("IsBlacklisted", func() {
		Context("given a blacklisted URL", func() {
			It("returns true", func() {
				isBlacklisted := urlBlacklistManager.IsBlacklisted("http://www.example.com/bad")
				Expect(isBlacklisted).To(BeTrue())
			})
		})

		Context("given a non-blacklisted URL", func() {
			It("returns false", func() {
				isBlacklisted := urlBlacklistManager.IsBlacklisted("http://www.example.com/good")
				Expect(isBlacklisted).To(BeFalse())
			})
		})
	})

	Describe("CheckUrl", func() {
		Context("given an invalid url", func() {
			It("returns an err", func() {
				_, err := urlBlacklistManager.CheckUrl("http://")

				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("(?i:incomplete url)"))
			})
		})
	})
})
