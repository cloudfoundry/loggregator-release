package config_test

import (
	"bytes"
	"metron/config"
	"strings"

	. "github.com/onsi/ginkgo"
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

				Expect(cfg.PreferredProtocol).To(BeEquivalentTo("udp"))
				Expect(cfg.BufferSize).To(Equal(100))

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
					PreferredProtocol:                "udp",
					BufferSize:                       100,
				}))
			})

			Context("returns error", func() {
				It("for invalid preferred protocol", func() {
					_, err := config.Parse(bytes.NewBufferString(`
				{
				  "PreferredProtocol": "NOT A REAL PROTOCOL",
				}
				`))

					Expect(err).To(HaveOccurred())
				})
			})

		})

		Context("returns error", func() {
			It("for invalid preferred protocol", func() {
				_, err := config.Parse(bytes.NewBufferString(`
				{"PreferredProtocol": "NOT A REAL PROTOCOL"}
				`))

				Expect(err).To(HaveOccurred())
			})
		})
	})
})
