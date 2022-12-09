package plumbing_test

import (
	"code.cloudfoundry.org/loggregator-release/src/testservers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/loggregator-release/src/plumbing"
)

var _ = Describe("TLS", func() {
	Context("NewClientCredentials", func() {
		It("returns transport credentials", func() {
			creds, err := plumbing.NewClientCredentials(
				testservers.LoggregatorTestCerts.Cert("doppler"),
				testservers.LoggregatorTestCerts.Key("doppler"),
				testservers.LoggregatorTestCerts.CA(),
				"doppler",
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(creds.Info().ServerName).To(Equal("doppler"))
		})

		It("returns an error with invalid certs", func() {
			creds, err := plumbing.NewClientCredentials(
				testservers.LoggregatorTestCerts.Cert("doppler"),
				testservers.LoggregatorTestCerts.Key("doppler"),
				testservers.LoggregatorTestCerts.Key("doppler"),
				"doppler",
			)
			Expect(err).To(HaveOccurred())
			Expect(creds).To(BeNil())
		})
	})

	Context("NewServerCredentials", func() {
		It("returns transport credentials", func() {
			_, err := plumbing.NewServerCredentials(
				testservers.LoggregatorTestCerts.Cert("doppler"),
				testservers.LoggregatorTestCerts.Key("doppler"),
				testservers.LoggregatorTestCerts.CA(),
			)
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns an error with invalid certs", func() {
			creds, err := plumbing.NewServerCredentials(
				testservers.LoggregatorTestCerts.Cert("doppler"),
				testservers.LoggregatorTestCerts.Key("doppler"),
				testservers.LoggregatorTestCerts.Key("doppler"),
			)
			Expect(err).To(HaveOccurred())
			Expect(creds).To(BeNil())
		})
	})
})
