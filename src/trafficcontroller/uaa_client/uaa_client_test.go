package uaa_client_test

import (
	"crypto/tls"
	"integration_tests/trafficcontroller/fake_uaa_server"
	"net/http"
	"net/http/httptest"
	"trafficcontroller/uaa_client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("UaaClient", func() {
	var (
		handler       = fake_uaa_server.FakeUaaHandler{}
		fakeUaaServer = httptest.NewTLSServer(&handler)

		transport *http.Transport
	)

	BeforeEach(func() {
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
		http.DefaultClient.Transport = transport
	})

	Context("when the user is an admin", func() {
		It("Determines permissions from correct credentials", func() {
			uaaClient := uaa_client.NewUaaClient(fakeUaaServer.URL, "bob", "yourUncle")

			authData, err := uaaClient.GetAuthData("iAmAnAdmin")
			Expect(err).ToNot(HaveOccurred())

			Expect(authData.HasPermission("doppler.firehose")).To(Equal(true))
			Expect(authData.HasPermission("uaa.not-admin")).To(Equal(false))

		})
	})

	Context("when the user is not an admin", func() {
		It("Determines permissions from correct credentials", func() {
			uaaClient := uaa_client.NewUaaClient(fakeUaaServer.URL, "bob", "yourUncle")

			authData, err := uaaClient.GetAuthData("iAmNotAnAdmin")
			Expect(err).ToNot(HaveOccurred())

			Expect(authData.HasPermission("doppler.firehose")).To(Equal(false))
			Expect(authData.HasPermission("uaa.not-admin")).To(Equal(true))
		})
	})

	Context("the token is expired", func() {
		It("returns the proper error", func() {
			uaaClient := uaa_client.NewUaaClient(fakeUaaServer.URL, "bob", "yourUncle")

			_, err := uaaClient.GetAuthData("expiredToken")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Token has expired"))
		})
	})

	Context("the token is invalid", func() {
		It("returns the proper error", func() {
			uaaClient := uaa_client.NewUaaClient(fakeUaaServer.URL, "bob", "yourUncle")

			_, err := uaaClient.GetAuthData("invalidToken")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Invalid token (could not decode): invalidToken"))
		})
	})

	Context("the server returns a 500 ", func() {
		It("returns the proper error", func() {
			uaaClient := uaa_client.NewUaaClient(fakeUaaServer.URL, "bob", "yourUncle")

			_, err := uaaClient.GetAuthData("500Please")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Unknown error occurred"))
		})
	})

	Context("the un/pwd is invalid", func() {
		It("returns the proper error", func() {
			uaaClient := uaa_client.NewUaaClient(fakeUaaServer.URL, "wrongUser", "yourUncle")

			_, err := uaaClient.GetAuthData("iAmAnAdmin")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Invalid username/password"))
		})
	})

	Context("insecure skip verify is false", func() {
		BeforeEach(func() {
			transport.TLSClientConfig.InsecureSkipVerify = false
		})

		It("returns an error status", func() {
			uaaClient := uaa_client.NewUaaClient(fakeUaaServer.URL, "bob", "yourUncle")

			_, err := uaaClient.GetAuthData("iAmAnAdmin")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(HaveSuffix("x509: certificate signed by unknown authority"))
		})
	})
})
