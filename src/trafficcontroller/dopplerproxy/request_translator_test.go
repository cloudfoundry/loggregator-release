package dopplerproxy_test

import (
	"trafficcontroller/dopplerproxy"

	"net/http"
	"net/url"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TranslateFromLegacyPath", func() {
	It("translates the /tail/ endpoint correctly", func() {
		request, _ := http.NewRequest("GET", "/tail/?app=my-app-id", nil)
		translatedRequest, err := dopplerproxy.TranslateFromLegacyPath(request)
		Expect(err).ToNot(HaveOccurred())
		Expect(translatedRequest.URL.Path).To(Equal("/apps/my-app-id/stream"))
	})

	It("translates the /dump/ endpoint correctly", func() {
		request, _ := http.NewRequest("GET", "/dump/?app=my-app-id", nil)
		translatedRequest, err := dopplerproxy.TranslateFromLegacyPath(request)
		Expect(err).ToNot(HaveOccurred())
		Expect(translatedRequest.URL.Path).To(Equal("/apps/my-app-id/recentlogs"))
	})

	It("translates the /recent endpoint correctly", func() {
		request, _ := http.NewRequest("GET", "/recent?app=my-app-id", nil)
		translatedRequest, err := dopplerproxy.TranslateFromLegacyPath(request)
		Expect(err).ToNot(HaveOccurred())
		Expect(translatedRequest.URL.Path).To(Equal("/apps/my-app-id/recentlogs"))
	})

	It("does nothing for /set-cookie", func() {
		request, _ := http.NewRequest("GET", "/set-cookie", nil)
		translatedRequest, err := dopplerproxy.TranslateFromLegacyPath(request)
		Expect(err).ToNot(HaveOccurred())
		Expect(translatedRequest).To(Equal(request))
	})

	It("returns an error for invalid paths", func() {
		request, _ := http.NewRequest("GET", "/invalid-path?app=my-app-id", nil)
		_, err := dopplerproxy.TranslateFromLegacyPath(request)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unexpected path"))
	})

	It("returns an error if ParseForm fails", func() {
		request, _ := http.NewRequest("GET", "/invalid-path?app=my-app-id", nil)
		request.URL.RawQuery = "asdf%asdf"
		_, err := dopplerproxy.TranslateFromLegacyPath(request)
		Expect(err).To(BeAssignableToTypeOf(*new(url.EscapeError)))
	})

	It("returns an error if there is no app ID", func() {
		request, _ := http.NewRequest("GET", "/dump/", nil)
		_, err := dopplerproxy.TranslateFromLegacyPath(request)
		Expect(err).To(Equal(dopplerproxy.MissingAppIdError))
	})
})
