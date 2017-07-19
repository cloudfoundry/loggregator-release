package plumbing_test

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"os"

	"code.cloudfoundry.org/loggregator/testservers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/loggregator/plumbing"
)

var _ = Describe("TLS", func() {
	Context("NewClientMutualTLSConfig", func() {
		It("builds a mutual auth tls config", func() {
			conf, err := plumbing.NewClientMutualTLSConfig(
				testservers.Cert("doppler.crt"),
				testservers.Cert("doppler.key"),
				testservers.Cert("loggregator-ca.crt"),
				"test-server-name",
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(conf.Certificates).To(HaveLen(1))
			Expect(conf.InsecureSkipVerify).To(BeFalse())
			Expect(conf.MinVersion).To(Equal(uint16(tls.VersionTLS12)))
			Expect(conf.CipherSuites).To(BeEmpty())

			Expect(string(conf.RootCAs.Subjects()[0])).To(ContainSubstring("loggregatorCA"))

			Expect(conf.ServerName).To(Equal("test-server-name"))
		})

		It("allows you to not specify a CA cert", func() {
			conf, err := plumbing.NewClientMutualTLSConfig(
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
			_, err := plumbing.NewClientMutualTLSConfig(
				"",
				"",
				testservers.Cert("loggregator-ca.crt"),
				"",
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("failed to load keypair: open : no such file or directory"))
		})

		It("returns an error when given invalid ca cert path", func() {
			_, err := plumbing.NewClientMutualTLSConfig(
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
			_, err := plumbing.NewClientMutualTLSConfig(
				testservers.Cert("doppler.crt"),
				testservers.Cert("doppler.key"),
				empty,
				"",
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("unable to load ca cert file"))
		})

		It("returns an error when the certificate is not signed by the CA", func() {
			_, err := plumbing.NewClientMutualTLSConfig(
				testservers.Cert("doppler.crt"),
				testservers.Cert("doppler.key"),
				wrongCA(),
				"",
			)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("NewServerMutalTLSConfig", func() {
		It("builds a mutual auth tls config", func() {
			conf, err := plumbing.NewServerMutualTLSConfig(
				testservers.Cert("doppler.crt"),
				testservers.Cert("doppler.key"),
				testservers.Cert("loggregator-ca.crt"),
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(conf.Certificates).To(HaveLen(1))
			Expect(conf.InsecureSkipVerify).To(BeFalse())
			Expect(conf.ClientAuth).To(Equal(tls.RequireAndVerifyClientCert))
			Expect(conf.MinVersion).To(Equal(uint16(tls.VersionTLS12)))
			Expect(string(conf.ClientCAs.Subjects()[0])).To(ContainSubstring("loggregatorCA"))
		})

		It("builds a config struct with default CIPHERs", func() {
			conf, err := plumbing.NewServerMutualTLSConfig(
				testservers.Cert("doppler.crt"),
				testservers.Cert("doppler.key"),
				testservers.Cert("loggregator-ca.crt"),
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(conf.CipherSuites).To(HaveLen(2))
			Expect(conf.CipherSuites).To(ConsistOf(
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			))
		})

		It("accepts configuration for CIPHER suites", func() {
			conf, err := plumbing.NewServerMutualTLSConfig(
				testservers.Cert("doppler.crt"),
				testservers.Cert("doppler.key"),
				testservers.Cert("loggregator-ca.crt"),
				plumbing.WithCipherSuites([]string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"}),
			)
			Expect(err).NotTo(HaveOccurred())

			Expect(conf.CipherSuites).To(ConsistOf(
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			))
		})

		It("ignores garbage CIPHERs in favor of valid ones", func() {
			conf, err := plumbing.NewServerMutualTLSConfig(
				testservers.Cert("doppler.crt"),
				testservers.Cert("doppler.key"),
				testservers.Cert("loggregator-ca.crt"),
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
				plumbing.NewServerMutualTLSConfig(
					testservers.Cert("doppler.crt"),
					testservers.Cert("doppler.key"),
					testservers.Cert("loggregator-ca.crt"),
					plumbing.WithCipherSuites([]string{}),
				)
			}).Should(Panic())
		})

		It("maintains the order of the ciphers provided", func() {
			conf, err := plumbing.NewServerMutualTLSConfig(
				testservers.Cert("doppler.crt"),
				testservers.Cert("doppler.key"),
				testservers.Cert("loggregator-ca.crt"),
				plumbing.WithCipherSuites([]string{
					"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
					"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
				}),
			)
			Expect(err).NotTo(HaveOccurred())

			Expect(conf.CipherSuites).To(Equal([]uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
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
		})
	})

	Context("NewClientCredentials", func() {
		It("returns transport credentials", func() {
			creds, err := plumbing.NewClientCredentials(
				testservers.Cert("doppler.crt"),
				testservers.Cert("doppler.key"),
				testservers.Cert("loggregator-ca.crt"),
				"doppler",
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(creds.Info().ServerName).To(Equal("doppler"))
		})

		It("returns an error with invalid certs", func() {
			creds, err := plumbing.NewClientCredentials(
				testservers.Cert("doppler.crt"),
				testservers.Cert("doppler.key"),
				testservers.Cert("doppler.key"),
				"doppler",
			)
			Expect(err).To(HaveOccurred())
			Expect(creds).To(BeNil())
		})
	})

	Context("NewServerCredentials", func() {
		It("returns transport credentials", func() {
			_, err := plumbing.NewServerCredentials(
				testservers.Cert("doppler.crt"),
				testservers.Cert("doppler.key"),
				testservers.Cert("loggregator-ca.crt"),
				plumbing.WithCipherSuites([]string{
					"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
				}),
			)
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns an error with invalid certs", func() {
			creds, err := plumbing.NewServerCredentials(
				testservers.Cert("doppler.crt"),
				testservers.Cert("doppler.key"),
				testservers.Cert("doppler.key"),
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

func wrongCA() string {
	someCA := `
-----BEGIN CERTIFICATE-----
MIIDKTCCAhGgAwIBAgIQKn+Y6URzoieLEs3Vc8e3PTANBgkqhkiG9w0BAQsFADA+
MQwwCgYDVQQGEwNVU0ExFjAUBgNVBAoTDUNsb3VkIEZvdW5kcnkxFjAUBgNVBAMT
DWxvZ2dyZWdhdG9yQ0EwHhcNMTcwNTAyMTgyNjM2WhcNMTgwNTAyMTgyNjM2WjA+
MQwwCgYDVQQGEwNVU0ExFjAUBgNVBAoTDUNsb3VkIEZvdW5kcnkxFjAUBgNVBAMT
DWxvZ2dyZWdhdG9yQ0EwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDl
FmaUqdn8XEJ8Uh8nhQSsqf/u0nDg1oyPlVk1XA8rwibn1ENoeqCyxPPfDIMfozt0
X5aRqB7Opgn66VQxdD9pIh2jRirvSCC15392VVRX1YJAVCk51zcmGSF7wNky++DG
VonpPdREyPO78joYtdysMkhwWUD+iDHFg4IbmKvklzJbBoVCjMsmqubqae5RhzZ2
vAOu2SP5jYYyjrvfC/pN41sWN9mYEXsLfVxwT6oFTKBxmCHNnJ6zqyJmxbz8JykJ
3Z08GovnKMGcYquZyxREk10Mz5hzor4/G53toUAbfDRfAuTUnsUXbJu3/5UB4VIw
gbRRAxNHsPYMu6iuG4vrAgMBAAGjIzAhMA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMB
Af8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQBMkum9b9Jh8kwzBUTgXMbtdRYf
Dnj4LeymTElBp0jsMHcEfKjyOVDtEUusmyhCbIaSPU1uyHRqFw4e5e83372JTazg
FGNs+nMEp3ph+dZdCQaNtO5GhUirC6AyJyvoBaO7enwTDWEMOBPynOvKbuW3fsBF
Z6No3WSX/53KcdPvJch6GWMcRoPRuMhkcTe5bASZYskC+td0PesuFTaBBHxEb9ij
en1e8KTEwXpj1NURmEluDy0KSyXicem4cYRBRbrVX0tliibgrUxj1pK4DxgW84+y
/9xLJDAaNKGHIxPT6Cw3TmIIfywGncHnI8/ObhPB9Cg3Xv6dMDemZVkFbuue
-----END CERTIFICATE-----`

	f, err := ioutil.TempFile("", "test_ca")
	if err != nil {
		panic(err)
	}

	_, err = f.Write([]byte(someCA))
	if err != nil {
		panic(err)
	}

	return f.Name()
}
