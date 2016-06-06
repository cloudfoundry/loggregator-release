package httpsetup_test

import (
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"trafficcontroller/httpsetup"
)

var _ = Describe("DefaultClient", func() {
	It("has a 20 second timeout", func() {
		Expect(http.DefaultClient.Timeout).To(Equal(20 * time.Second))
	})

	Describe("DefaultClient.Transport", func() {
		var transport *http.Transport

		BeforeEach(func() {
			var ok bool
			transport, ok = http.DefaultClient.Transport.(*http.Transport)
			Expect(ok).To(BeTrue(), "Expected http.DefaultClient.Transport to be a *http.Transport")
		})

		It("has a 10 second handshake timeout", func() {
			Expect(transport.TLSHandshakeTimeout).To(Equal(10 * time.Second))
		})

		It("enables DisableKeepAlives", func() {
			Expect(transport.DisableKeepAlives).To(BeTrue())
		})

		Describe("SetInsecureSkipVerify", func() {
			It("sets InsecureSkipVerify on TLSClientConfig", func() {
				httpsetup.SetInsecureSkipVerify(true)
				Expect(transport.TLSClientConfig.InsecureSkipVerify).To(BeTrue())
			})
		})
	})
})
