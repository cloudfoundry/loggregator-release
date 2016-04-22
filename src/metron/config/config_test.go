package config_test

import (
	"bytes"
	"fmt"
	"metron/config"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {

	Describe("ParseConfig", func() {
		var (
			configFile string
		)
		Context("returns an error", func() {

			It("returns error for invalid config file path", func() {
				configFile = "./fixtures/IDoNotExist.json"
				_, err := config.ParseConfig(configFile)
				Expect(err).To(HaveOccurred())
			})

			It("returns error for invalid json", func() {
				configFile = "./fixtures/invalid_metron.json"
				_, err := config.ParseConfig(configFile)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("parses config properly", func() {
			It("returns proper config", func() {
				configFile = "./fixtures/metron.json"
				cfg, err := config.ParseConfig(configFile)
				Expect(err).ToNot(HaveOccurred())

				Expect(cfg.Index).To(Equal(uint(0)))
				Expect(cfg.Job).To(Equal("job-name"))
				Expect(cfg.Zone).To(Equal("z1"))
				Expect(cfg.Deployment).To(Equal("deployment-name"))

				Expect(cfg.EtcdUrls).To(HaveLen(1))
				Expect(cfg.EtcdMaxConcurrentRequests).To(Equal(1))
				Expect(cfg.EtcdQueryIntervalMilliseconds).To(Equal(100))

				Expect(cfg.SharedSecret).To(Equal("shared_secret"))

				Expect(cfg.IncomingUDPPort).To(Equal(51161))
				Expect(cfg.LoggregatorDropsondePort).To(Equal(3457))

				Expect(cfg.MetricBatchIntervalMilliseconds).To(BeEquivalentTo(20))
				Expect(cfg.RuntimeStatsIntervalMilliseconds).To(BeEquivalentTo(15))
				Expect(cfg.Syslog).To(Equal("syslog.namespace"))

				Expect(cfg.TCPBatchSizeBytes).To(BeEquivalentTo(1024))
				Expect(cfg.TCPBatchIntervalMilliseconds).To(BeEquivalentTo(10))

				Expect(cfg.Protocols).To(Equal([]config.Protocol{"tls", "udp"}))

				Expect(cfg.TLSConfig).To(Equal(config.TLSConfig{
					CertFile: "./fixtures/client.crt",
					KeyFile:  "./fixtures/client.key",
					CAFile:   "./fixtures/ca.crt",
				}))
			})
		})

		Describe("Parse", func() {
			It("sets defaults", func() {
				cfg, err := config.Parse(strings.NewReader("{}"))
				Expect(err).ToNot(HaveOccurred())

				Expect(cfg).To(Equal(&config.Config{
					TCPBatchIntervalMilliseconds:     100,
					TCPBatchSizeBytes:                10240,
					MetricBatchIntervalMilliseconds:  5000,
					RuntimeStatsIntervalMilliseconds: 15000,
					Protocols:                        []config.Protocol{"tcp", "udp"},
				}))
			})

			Context("with valid protocols", func() {
				DescribeTable("doesn't return error", func(proto string) {
					_, err := config.Parse(bytes.NewBufferString(fmt.Sprintf(`
						{
							"Protocols": ["%s"]
						}
					`, proto)))
					Expect(err).ToNot(HaveOccurred())
				},
					Entry("udp", "udp"),
					Entry("tcp", "tcp"),
					Entry("tls", "tls"),
				)
			})

			Context("with invalid PreferredProtocol", func() {
				var err error

				BeforeEach(func() {
					_, err = config.Parse(bytes.NewBufferString(`
						{
							"Protocols": ["NOT A REAL PROTOCOL"]
						}
					`))
				})

				It("returns error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("Invalid protocol: NOT A REAL PROTOCOL"))
				})
			})

			Context("with empty Protocols", func() {
				var err error

				BeforeEach(func() {
					_, err = config.Parse(bytes.NewBufferString(`
						{
							"Protocols": []
						}
					`))
				})

				It("returns error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("Metron cannot start without protocols"))
				})
			})
		})
	})
})
