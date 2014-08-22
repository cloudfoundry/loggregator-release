package main_test

import (
	"trafficcontroller"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Main", func() {
	Describe("ParseConfig", func() {
		var (
			logLevel    = false
			logFilePath = "./test_assets/stdout.log"
		)

		It("reads the outgoing dropsonde port from config", func() {
			configFile := "./test_assets/minimal_loggregator_trafficcontroller.json"

			var config *main.Config

			config, _, _ = main.ParseConfig(&logLevel, &configFile, &logFilePath)

			Expect(config.OutgoingDropsondePort).To(Equal(uint32(4566)))
		})

		Context("with empty Loggregator ports", func() {
			It("uses defaults", func() {
				configFile := "./test_assets/minimal_loggregator_trafficcontroller.json"

				var config *main.Config

				config, _, _ = main.ParseConfig(&logLevel, &configFile, &logFilePath)

				Expect(config.IncomingPort).To(Equal(uint32(8765)))
				Expect(config.LoggregatorIncomingPort).To(Equal(uint32(8765)))
				Expect(config.OutgoingPort).To(Equal(uint32(4567)))
				Expect(string(config.LoggregatorOutgoingPort)).To(Equal(string(uint32(4567))))
			})
		})

		Context("with specified Loggregator ports", func() {
			It("uses specified ports", func() {
				configFile := "./test_assets/loggregator_trafficcontroller.json"

				var config *main.Config

				config, _, _ = main.ParseConfig(&logLevel, &configFile, &logFilePath)

				Expect(config.IncomingPort).To(Equal(uint32(8765)))
				Expect(config.LoggregatorIncomingPort).To(Equal(uint32(8766)))
				Expect(config.OutgoingPort).To(Equal(uint32(4567)))
				Expect(config.LoggregatorOutgoingPort).To(Equal(uint32(4568)))
			})
		})

		Context("without ETCD/heartbeat specific configuration", func() {
			It("uses defaults", func() {
				configFile := "./test_assets/minimal_loggregator_trafficcontroller.json"

				var config *main.Config

				config, _, _ = main.ParseConfig(&logLevel, &configFile, &logFilePath)

				Expect(config.JobName).To(Equal("loggregator_trafficcontroller"))
				Expect(config.JobIndex).To(Equal(0))
				Expect(config.EtcdMaxConcurrentRequests).To(Equal(10))
			})
		})

		Context("with ETCD/heartbeat specific configuration", func() {
			It("uses specified properties", func() {
				configFile := "./test_assets/loggregator_trafficcontroller.json"

				var config *main.Config

				config, _, _ = main.ParseConfig(&logLevel, &configFile, &logFilePath)

				Expect(config.JobName).To(Equal("trafficcontroller"))
				Expect(config.JobIndex).To(Equal(3))
				Expect(config.EtcdMaxConcurrentRequests).To(Equal(5))
				Expect(config.EtcdUrls).To(ConsistOf([]string{"http://127.0.0.1:4001", "http://127.0.0.1:4002"}))
			})
		})
	})
})
