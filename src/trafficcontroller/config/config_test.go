package config_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"trafficcontroller/config"
)

var _ = Describe("Config", func() {
	Describe("ParseConfig", func() {
		It("reads the outgoing dropsonde port from config", func() {
			configFile := "./fixtures/minimal_loggregator_trafficcontroller.json"

			c, err := config.ParseConfig(configFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(c.OutgoingDropsondePort).To(Equal(uint32(4566)))
		})

		Context("without ETCD/heartbeat specific configuration", func() {
			It("uses defaults", func() {
				configFile := "./fixtures/minimal_loggregator_trafficcontroller.json"

				c, err := config.ParseConfig(configFile)
				Expect(err).ToNot(HaveOccurred())
				Expect(c.JobName).To(Equal("loggregator_trafficcontroller"))
				Expect(c.Index).To(BeEmpty())
				Expect(c.EtcdMaxConcurrentRequests).To(Equal(10))
			})
		})

		Context("with ETCD/heartbeat specific configuration", func() {
			It("uses specified properties", func() {
				configFile := "./fixtures/loggregator_trafficcontroller.json"

				c, err := config.ParseConfig(configFile)
				Expect(err).ToNot(HaveOccurred())
				Expect(c.JobName).To(Equal("trafficcontroller"))
				Expect(c.Index).To(Equal("3"))
				Expect(c.EtcdMaxConcurrentRequests).To(Equal(5))
				Expect(c.EtcdUrls).To(ConsistOf([]string{"http://127.0.0.1:4001", "http://127.0.0.1:4002"}))
			})
		})

		Context("with ETCD TLS enabled", func() {
			It("errors when TLS is required but cert/key files are not provided", func() {
				configFile := "./fixtures/bad_etcd_tls.json"

				_, err := config.ParseConfig(configFile)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("invalid etcd TLS client configuration"))
			})
		})

		Context("without MonitorIntervalSeconds", func() {
			It("defaults MonitorIntervalSeconds to 60 seconds", func() {
				configFile := "./fixtures/loggregator_trafficcontroller.json"

				c, err := config.ParseConfig(configFile)
				Expect(err).ToNot(HaveOccurred())
				Expect(c.MonitorIntervalSeconds).To(Equal(uint(60)))
			})
		})

		Context("without SecurityEventLog", func() {
			It("defaults SecurityEventLog to empty string", func() {
				configFile := "./fixtures/minimal_loggregator_trafficcontroller.json"

				c, err := config.ParseConfig(configFile)
				Expect(err).ToNot(HaveOccurred())
				Expect(c.SecurityEventLog).To(Equal(""))
			})
		})

		Context("with SecurityEventLog", func() {
			It("uses specified properties", func() {
				configFile := "./fixtures/loggregator_trafficcontroller.json"

				c, err := config.ParseConfig(configFile)
				Expect(err).ToNot(HaveOccurred())
				Expect(c.SecurityEventLog).To(Equal("access.log"))
			})
		})

		Context("without Metron host and port", func() {
			It("uses defaults", func() {
				configFile := "./fixtures/minimal_loggregator_trafficcontroller.json"

				c, err := config.ParseConfig(configFile)
				Expect(err).ToNot(HaveOccurred())
				Expect(c.MetronHost).To(Equal("127.0.0.1"))
				Expect(c.MetronPort).To(Equal(3457))
			})
		})

		Context("without grpc port", func() {
			It("uses defaults", func() {
				configFile := "./fixtures/minimal_loggregator_trafficcontroller.json"

				c, err := config.ParseConfig(configFile)
				Expect(err).ToNot(HaveOccurred())
				Expect(c.GRPC.Port).To(Equal(uint16(8082)))
			})
		})

		Context("with Metron host and port", func() {
			It("uses specified properties", func() {
				configFile := "./fixtures/loggregator_trafficcontroller.json"

				c, err := config.ParseConfig(configFile)
				Expect(err).ToNot(HaveOccurred())
				Expect(c.MetronHost).To(Equal("foo.bar"))
				Expect(c.MetronPort).To(Equal(7543))
			})
		})

		Context("with PPROFPort", func() {
			It("uses specified value", func() {
				configFile := "./fixtures/loggregator_trafficcontroller.json"

				c, err := config.ParseConfig(configFile)
				Expect(err).ToNot(HaveOccurred())
				Expect(c.PPROFPort).To(Equal(uint32(666)))
			})
		})

	})
})
