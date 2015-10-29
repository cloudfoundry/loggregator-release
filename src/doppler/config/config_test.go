package config_test

import (
	"doppler/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {

	var (
		configFile string
	)

	Context("with minimal config", func() {
		BeforeEach(func() {
			configFile = "./fixtures/minimal_doppler.json"
		})

		It("defaults to empty blacklist", func() {
			config, _ := config.ParseConfig(configFile)

			Expect(config.BlackListIps).To(BeNil())
		})

		It("returns proper config", func() {
			config, err := config.ParseConfig(configFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(config.DropsondeIncomingMessagesPort).To(Equal(uint32(3456)))
			Expect(config.OutgoingPort).To(Equal(uint32(8080)))
			Expect(config.MessageDrainBufferSize).To(Equal(uint(100)))
			Expect(config.MonitorIntervalSeconds).To(BeEquivalentTo(60))
			Expect(config.EnableTLSTransport).To(BeFalse())
		})
	})

	Context("With EnableTLSTransport", func() {
		It("generates the cert for the tls config", func() {
			configFile = "./fixtures/doppler.json"
			cfg, err := config.ParseConfig(configFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.EnableTLSTransport).To(BeTrue())
			Expect(cfg.TLSListenerConfig).To(Equal(config.TLSListenerConfig{
				Port:     8766,
				CertFile: "./fixtures/server.crt",
				KeyFile:  "./fixtures/server.key",
				CAFile:   "./fixtures/loggregator-ca.crt",
			}))
		})

		It("errors out if no cert or key files is provided", func() {
			configFile = "./fixtures/dopplerNoTLSConfig.json"
			_, err := config.ParseConfig(configFile)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("invalid TLS listener configuration"))
		})
	})

	Context("with full config", func() {
		BeforeEach(func() {
			configFile = "./fixtures/doppler.json"
		})

		It("returns proper config", func() {
			config, err := config.ParseConfig(configFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(config.DropsondeIncomingMessagesPort).To(Equal(uint32(8765)))
			Expect(config.OutgoingPort).To(Equal(uint32(4567)))
			Expect(config.MessageDrainBufferSize).To(Equal(uint(100)))
			Expect(config.BlackListIps[0].Start).To(Equal("127.0.0.0"))
			Expect(config.BlackListIps[0].End).To(Equal("127.0.0.2"))
			Expect(config.BlackListIps[1].Start).To(Equal("127.0.1.12"))
			Expect(config.BlackListIps[1].End).To(Equal("127.0.1.15"))
			Expect(config.MonitorIntervalSeconds).To(BeEquivalentTo(1))
			Expect(config.TLSListenerConfig).ToNot(BeNil())
		})

	})
})
