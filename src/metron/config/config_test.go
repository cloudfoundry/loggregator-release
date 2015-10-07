package config_test

import (
	"metron/config"

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
			config, err := config.ParseConfig(configFile)

			Expect(err).ToNot(HaveOccurred())
			Expect(config.Index).To(Equal(uint(0)))
			Expect(config.Job).To(Equal("job-name"))
			Expect(config.Zone).To(Equal("z1"))
			Expect(config.Deployment).To(Equal("deployment-name"))

			Expect(config.EtcdUrls).To(HaveLen(1))
			Expect(config.EtcdMaxConcurrentRequests).To(Equal(1))
			Expect(config.EtcdQueryIntervalMilliseconds).To(Equal(100))

			Expect(config.SharedSecret).To(Equal("shared_secret"))

			Expect(config.DropsondeIncomingMessagesPort).To(Equal(51161))
			Expect(config.LoggregatorDropsondePort).To(Equal(3457))

			Expect(config.MetricBatchIntervalSeconds).To(BeEquivalentTo(15))
			Expect(config.Syslog).To(Equal("syslog.namespace"))
		})
	})
})
