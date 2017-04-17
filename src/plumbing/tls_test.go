package plumbing_test

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"os"
	"testservers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"plumbing"
)

var _ = Describe("TLS", func() {
	Context("NewMutalTLSConfig", func() {
		It("builds a config struct with default CIPHERs", func() {
			conf, err := plumbing.NewMutualTLSConfig(
				testservers.Cert("doppler.crt"),
				testservers.Cert("doppler.key"),
				testservers.Cert("loggregator-ca.crt"),
				"test-server-name",
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(conf.Certificates).To(HaveLen(1))
			Expect(conf.InsecureSkipVerify).To(BeFalse())
			Expect(conf.ClientAuth).To(Equal(tls.RequireAndVerifyClientCert))
			Expect(conf.MinVersion).To(Equal(uint16(tls.VersionTLS12)))
			Expect(conf.CipherSuites).To(ConsistOf(
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			))

			Expect(string(conf.RootCAs.Subjects()[0])).To(ContainSubstring("loggregatorCA"))
			Expect(string(conf.ClientCAs.Subjects()[0])).To(ContainSubstring("loggregatorCA"))

			Expect(conf.ServerName).To(Equal("test-server-name"))
		})

		It("allows you to not specify a CA cert", func() {
			conf, err := plumbing.NewMutualTLSConfig(
				testservers.Cert("doppler.crt"),
				testservers.Cert("doppler.key"),
				"",
				"",
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(conf.RootCAs).To(BeNil())
			Expect(conf.ClientCAs).To(BeNil())
		})

		It("returns an error when given invalid cert/key paths", func() {
			_, err := plumbing.NewMutualTLSConfig(
				"",
				"",
				testservers.Cert("loggregator-ca.crt"),
				"",
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("failed to load keypair: open : no such file or directory"))
		})

		It("returns an error when given invalid ca cert path", func() {
			_, err := plumbing.NewMutualTLSConfig(
				testservers.Cert("doppler.crt"),
				testservers.Cert("doppler.key"),
				"/file/that/does/not/exist",
				"",
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("failed to read ca cert file: open /file/that/does/not/exist: no such file or directory"))
		})

		It("returns an error when given invalid ca cert file", func() {
			empty := writeFile("")
			defer func() {
				err := os.Remove(empty)
				Expect(err).ToNot(HaveOccurred())
			}()
			_, err := plumbing.NewMutualTLSConfig(
				testservers.Cert("doppler.crt"),
				testservers.Cert("doppler.key"),
				empty,
				"",
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("unable to load ca cert file"))
		})

		It("returns an error when the certificate is not signed by the CA", func() {
			_, err := plumbing.NewMutualTLSConfig(
				testservers.Cert("doppler.crt"),
				testservers.Cert("doppler.key"),
				testservers.Cert("doppler.crt"),
				"",
			)
			Expect(err).To(HaveOccurred())
			_, ok := err.(plumbing.CASignatureError)
			Expect(ok).To(BeTrue())
		})

		It("accepts configuration for CIPHER suites", func() {
			conf, err := plumbing.NewMutualTLSConfig(
				testservers.Cert("doppler.crt"),
				testservers.Cert("doppler.key"),
				testservers.Cert("loggregator-ca.crt"),
				"test-server-name",
				plumbing.WithCipherSuites([]string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"}),
			)
			Expect(err).NotTo(HaveOccurred())

			Expect(conf.CipherSuites).To(ConsistOf(
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			))
		})

		It("ignores garbage CIPHERs in favor of valid ones", func() {
			conf, err := plumbing.NewMutualTLSConfig(
				testservers.Cert("doppler.crt"),
				testservers.Cert("doppler.key"),
				testservers.Cert("loggregator-ca.crt"),
				"test-server-name",
				plumbing.WithCipherSuites([]string{
					"GARBAGE",
					"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
				}),
			)
			Expect(err).NotTo(HaveOccurred())

			Expect(conf.CipherSuites).To(ConsistOf(
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			))
		})

		It("panics if no ciphers are provided", func() {
			Expect(func() {
				plumbing.NewMutualTLSConfig(
					testservers.Cert("doppler.crt"),
					testservers.Cert("doppler.key"),
					testservers.Cert("loggregator-ca.crt"),
					"test-server-name",
					plumbing.WithCipherSuites([]string{}),
				)
			}).Should(Panic())
		})

		It("maintains the order of the ciphers provided", func() {
			conf, err := plumbing.NewMutualTLSConfig(
				testservers.Cert("doppler.crt"),
				testservers.Cert("doppler.key"),
				testservers.Cert("loggregator-ca.crt"),
				"test-server-name",
				plumbing.WithCipherSuites([]string{
					"TLS_RSA_WITH_RC4_128_SHA",
					"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
				}),
			)
			Expect(err).NotTo(HaveOccurred())

			Expect(conf.CipherSuites).To(Equal([]uint16{
				tls.TLS_RSA_WITH_RC4_128_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			}))
		})
	})

	Context("NewTLSConfig", func() {
		It("returns basic TLS config", func() {
			tlsConf := plumbing.NewTLSConfig()

			Expect(tlsConf.InsecureSkipVerify).To(BeFalse())
			Expect(tlsConf.ClientAuth).To(Equal(tls.NoClientCert))
			Expect(tlsConf.MinVersion).To(Equal(uint16(tls.VersionTLS12)))
			Expect(tlsConf.CipherSuites).To(Equal([]uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			}))
		})

		It("accepts configuration for CIPHER suites", func() {
			conf := plumbing.NewTLSConfig(
				plumbing.WithCipherSuites([]string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"}),
			)

			Expect(conf.CipherSuites).To(ConsistOf(
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			))
		})

		It("ignores garbage CIPHERs in favor of valid ones", func() {
			conf := plumbing.NewTLSConfig(
				plumbing.WithCipherSuites([]string{
					"GARBAGE",
					"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
				}),
			)

			Expect(conf.CipherSuites).To(ConsistOf(
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			))
		})

		It("panics if no ciphers are provided", func() {
			Expect(func() {
				plumbing.NewTLSConfig(
					plumbing.WithCipherSuites([]string{}),
				)
			}).Should(Panic())
		})

		It("maintains the order of the ciphers provided", func() {
			conf := plumbing.NewTLSConfig(
				plumbing.WithCipherSuites([]string{
					"TLS_RSA_WITH_RC4_128_SHA",
					"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
				}),
			)

			Expect(conf.CipherSuites).To(Equal([]uint16{
				tls.TLS_RSA_WITH_RC4_128_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			}))
		})

	})

	Context("NewCredentials", func() {
		It("returns transport credentials", func() {
			creds, err := plumbing.NewCredentials(
				testservers.Cert("doppler.crt"),
				testservers.Cert("doppler.key"),
				testservers.Cert("loggregator-ca.crt"),
				"doppler",
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(creds.Info().ServerName).To(Equal("doppler"))
		})

		It("returns an error with invalid certs", func() {
			creds, err := plumbing.NewCredentials(
				testservers.Cert("doppler.crt"),
				testservers.Cert("doppler.key"),
				testservers.Cert("doppler.key"),
				"doppler",
			)
			Expect(err).To(HaveOccurred())
			Expect(creds).To(BeNil())
		})
	})
})

func writeFile(data string) string {
	f, err := ioutil.TempFile("", "")
	Expect(err).ToNot(HaveOccurred())
	_, err = fmt.Fprintf(f, data)
	Expect(err).ToNot(HaveOccurred())
	return f.Name()
}
