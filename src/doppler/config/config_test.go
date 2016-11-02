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

		It("returns proper config", func() {
			cfg, err := config.ParseConfig(configFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.IncomingUDPPort).To(Equal(uint32(3456)))
			Expect(cfg.IncomingTCPPort).To(Equal(uint32(3457)))
			Expect(cfg.OutgoingPort).To(Equal(uint32(8080)))
			Expect(cfg.MessageDrainBufferSize).To(Equal(uint(100)))
			Expect(cfg.MonitorIntervalSeconds).To(BeEquivalentTo(60))
			Expect(cfg.EnableTLSTransport).To(BeFalse())
			Expect(cfg.EtcdRequireTLS).To(BeFalse())
			Expect(cfg.MetricBatchIntervalMilliseconds).To(BeEquivalentTo(5000))
			Expect(cfg.GRPC.Port).To(BeEquivalentTo(4567))
			Expect(cfg.GRPC.CAFile).To(Equal("some-ca-file-path"))
			Expect(cfg.GRPC.KeyFile).Should(Equal("some-key-file-path"))
			Expect(cfg.GRPC.CertFile).Should(Equal("some-cert-file-path"))
		})

		It("defaults to empty blacklist", func() {
			config, _ := config.ParseConfig(configFile)

			Expect(config.BlackListIps).To(BeNil())
		})
	})

	Context("with less than minimal config", func() {
		It("errors out when GRPC is not provided", func() {
			confData := []byte(`{
				"OutgoingPort": 8080,
				"MessageDrainBufferSize": 100,
				"SinkSkipCertVerify": false,
				"Index": "0",
				"MaxRetainedLogMessages": 10,
				"SharedSecret": "mysecret",
				"Syslog"  : "",
				"ContainerMetricTTLSeconds": 120,
				"SinkInactivityTimeoutSeconds": 120,
				"EnableTLSTransport": false
			}`)
			_, err := config.Parse(confData)
			Expect(err).To(HaveOccurred())
		})

		It("errors out when GRPCTLSConfig is not provided", func() {
			confData := []byte(`{
				"OutgoingPort": 8080,
				"MessageDrainBufferSize": 100,
				"SinkSkipCertVerify": false,
				"Index": "0",
				"MaxRetainedLogMessages": 10,
				"SharedSecret": "mysecret",
				"Syslog"  : "",
				"ContainerMetricTTLSeconds": 120,
				"SinkInactivityTimeoutSeconds": 120,
				"EnableTLSTransport": false,
				"GRPC":{
                    "Port": 1234
                }
			}`)
			_, err := config.Parse(confData)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("with EnableTLSTransport", func() {
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

	Context("with EtcdRequireTLS", func() {
		It("generates the cert for the tls config", func() {
			configFile = "./fixtures/doppler.json"
			cfg, err := config.ParseConfig(configFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.EtcdRequireTLS).To(BeTrue())
			Expect(cfg.EtcdTLSClientConfig).To(Equal(config.EtcdTLSClientConfig{
				CertFile: "./fixtures/etcd-client.crt",
				KeyFile:  "./fixtures/etcd-client.key",
				CAFile:   "./fixtures/etcd-ca.crt",
			}))
		})

		It("errors out if no cert or key files is provided", func() {
			configFile = "./fixtures/dopplerNoEtcdTLSConfig.json"
			_, err := config.ParseConfig(configFile)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("invalid etcd TLS client configuration"))
		})
	})

	Context("with full config", func() {
		BeforeEach(func() {
			configFile = "./fixtures/doppler.json"
		})

		It("returns proper config", func() {
			config, err := config.ParseConfig(configFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(config.IncomingUDPPort).To(Equal(uint32(8765)))
			Expect(config.IncomingTCPPort).To(Equal(uint32(8767)))
			Expect(config.OutgoingPort).To(Equal(uint32(4567)))
			Expect(config.MessageDrainBufferSize).To(Equal(uint(100)))
			Expect(config.BlackListIps[0].Start).To(Equal("127.0.0.0"))
			Expect(config.BlackListIps[0].End).To(Equal("127.0.0.2"))
			Expect(config.BlackListIps[1].Start).To(Equal("127.0.1.12"))
			Expect(config.BlackListIps[1].End).To(Equal("127.0.1.15"))
			Expect(config.MonitorIntervalSeconds).To(BeEquivalentTo(1))
			Expect(config.TLSListenerConfig).ToNot(BeNil())
			Expect(config.WebsocketWriteTimeoutSeconds).To(Equal(60))
			Expect(config.MetricBatchIntervalMilliseconds).To(BeEquivalentTo(15000))
			Expect(config.PPROFPort).To(BeEquivalentTo(666))
		})
	})
})
