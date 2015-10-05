package main_test

import (
	"errors"
	"time"

	"doppler"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/storeadapter/fakestoreadapter"
	"github.com/pivotal-golang/localip"

	"doppler/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Main", func() {

	Describe("ParseConfig", func() {

		var (
			configFile  string
			logLevel    = false
			logFilePath = "./test_assets/stdout.log"
		)

		Context("with minimal config", func() {
			BeforeEach(func() {
				configFile = "./test_assets/minimal_doppler.json"
			})

			It("defaults to empty blacklist", func() {
				config, _ := main.ParseConfig(&logLevel, &configFile, &logFilePath)

				Expect(config.BlackListIps).To(BeNil())
			})

			It("returns proper config", func() {
				config, _ := main.ParseConfig(&logLevel, &configFile, &logFilePath)

				Expect(config.DropsondeIncomingMessagesPort).To(Equal(uint32(3456)))
				Expect(config.OutgoingPort).To(Equal(uint32(8080)))
				Expect(config.MessageDrainBufferSize).To(Equal(uint(100)))
				Expect(config.MonitorIntervalSeconds).To(BeEquivalentTo(60))
				Expect(config.EnableTLSTransport).To(BeFalse())
				Expect(config.TLSListenerConfig).To(BeNil())
			})
		})

		Context("With EnableTLSTransport", func() {
			It("generates the cert for the tls config", func() {
				configFile = "./test_assets/doppler.json"

				config, _ := main.ParseConfig(&logLevel, &configFile, &logFilePath)

				Expect(config.EnableTLSTransport).To(BeTrue())
				Expect(config.TLSListenerConfig.Cert.Certificate).ToNot(HaveLen(0))
			})

		})

		Context("with full config", func() {
			BeforeEach(func() {
				configFile = "./test_assets/doppler.json"
			})

			It("returns proper config", func() {
				config, _ := main.ParseConfig(&logLevel, &configFile, &logFilePath)

				Expect(config.DropsondeIncomingMessagesPort).To(Equal(uint32(8765)))
				Expect(config.OutgoingPort).To(Equal(uint32(4567)))
				Expect(config.MessageDrainBufferSize).To(Equal(uint(100)))
				Expect(config.BlackListIps[0].Start).To(Equal("127.0.0.0"))
				Expect(config.BlackListIps[0].End).To(Equal("127.0.0.2"))
				Expect(config.BlackListIps[1].Start).To(Equal("127.0.1.12"))
				Expect(config.BlackListIps[1].End).To(Equal("127.0.1.15"))
				Expect(config.MonitorIntervalSeconds).To(BeEquivalentTo(1))
				Expect(config.TLSListenerConfig).ToNot(BeNil())
				Expect(config.TLSListenerConfig.InsecureSkipVerify).To(BeTrue())
			})

			It("sets up logger", func() {
				logLevel = false
				_, logger := main.ParseConfig(&logLevel, &configFile, &logFilePath)
				Expect(logger.Level().String()).To(Equal("info"))

				logLevel = true
				_, logger = main.ParseConfig(&logLevel, &configFile, &logFilePath)
				Expect(logger.Level().String()).To(Equal("debug"))
			})
		})
	})

	Describe("StartHeartbeats", func() {
		var adapter *fakestoreadapter.FakeStoreAdapter

		BeforeEach(func() {
			adapter = fakestoreadapter.New()
		})

		Context("when store adapter is nil", func() {
			var conf config.Config
			var localIp string

			BeforeEach(func() {
				localIp, _ = localip.LocalIP()
				conf = config.Config{
					JobName: "doppler_z1",
					Index:   0,
					EtcdMaxConcurrentRequests: 10,
					EtcdUrls:                  []string{"test:123", "test:456"},
					Zone:                      "z1",
					DropsondeIncomingMessagesPort: 1234,
				}
			})

			It("should panic", func() {
				Expect(func() {
					main.StartHeartbeats(localIp, time.Second, &conf, nil, loggertesthelper.Logger())
				}).Should(Panic())
			})
		})

		Context("with a valid ETCD conig", func() {
			var conf config.Config
			var localIp string

			BeforeEach(func() {
				localIp, _ = localip.LocalIP()
				conf = config.Config{
					JobName: "doppler_z1",
					Index:   0,
					EtcdMaxConcurrentRequests: 10,
					EtcdUrls:                  []string{"test:123", "test:456"},
					Zone:                      "z1",
					DropsondeIncomingMessagesPort: 1234,
				}
			})

			It("sends a heartbeat to etcd", func() {
				main.StartHeartbeats(localIp, time.Second, &conf, adapter, loggertesthelper.Logger())
				Expect(adapter.GetMaintainedNodeName()).To(Equal("/healthstatus/doppler/z1/doppler_z1/0"))

				Expect(adapter.MaintainedNodeValue).To(Equal([]byte(localIp)))
			})

			Context("when there is an error", func() {
				It("panics", func() {
					adapter.MaintainNodeError = errors.New("error")
					Expect(func() { main.StartHeartbeats(localIp, time.Second, &conf, adapter, loggertesthelper.Logger()) }).To(Panic())
				})
			})
		})

		Context("without a valid ETCD config", func() {
			It("does not send a heartbeat", func() {
				conf := config.Config{
					JobName: "doppler_z1",
					Index:   0,
					EtcdMaxConcurrentRequests: 10,
				}

				localIp, _ := localip.LocalIP()
				main.StartHeartbeats(localIp, time.Second, &conf, adapter, loggertesthelper.Logger())
				Expect(adapter.GetMaintainedNodeName()).To(BeEmpty())
			})
		})
	})

})
