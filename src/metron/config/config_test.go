package config_test

import (
	"metron/config"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {

	Context("Parse config", func() {
		var (
			configFile string
		)

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

			Expect(cfg.DropsondeIncomingMessagesPort).To(Equal(51161))
			Expect(cfg.LoggregatorDropsondePort).To(Equal(3457))

			Expect(cfg.MetricBatchIntervalSeconds).To(BeEquivalentTo(20))
			Expect(cfg.Syslog).To(Equal("syslog.namespace"))

			Expect(cfg.PreferredProtocol).To(Equal("udp"))
			Expect(cfg.BufferSize).To(Equal(100))

			Expect(cfg.TLSConfig).To(Equal(config.TLSConfig{
				CertFile: "./fixtures/client.crt",
				KeyFile:  "./fixtures/client.key",
				CAFile:   "./fixtures/ca.crt",
			}))
		})

		It("sets defaults", func() {
			cfg, err := config.Parse(strings.NewReader("{}"))
			Expect(err).ToNot(HaveOccurred())

			Expect(cfg).To(Equal(&config.Config{
				MetricBatchIntervalSeconds: 15,
				PreferredProtocol:          "udp",
				BufferSize:                 100,
			}))
		})
	})
})
