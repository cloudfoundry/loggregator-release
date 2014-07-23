package main_test

import (
	"errors"
	"time"

	"github.com/cloudfoundry/loggregatorlib/cfcomponent/localip"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/fakestoreadapter"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"loggregator"
)

//Test ParseConfig
var _ = Describe("Main", func() {

	Describe("ParseConfig", func() {

		var (
			configFile  string
			logLevel    = false
			logFilePath = "./test_assets/stdout.log"
		)

		Context("with minimal config", func() {
			BeforeEach(func() {
				configFile = "./test_assets/minimal_loggregator.json"
			})

			It("defaults to empty blacklist", func() {
				config, _ := main.ParseConfig(&logLevel, &configFile, &logFilePath)

				Expect(config.BlackListIps).To(BeNil())
			})

			It("returns proper config", func() {
				config, _ := main.ParseConfig(&logLevel, &configFile, &logFilePath)

				Expect(config.IncomingPort).To(Equal(uint32(3456)))
				Expect(config.OutgoingPort).To(Equal(uint32(8080)))
				Expect(config.WSMessageBufferSize).To(Equal(uint(100)))
			})
		})

		Context("with full config", func() {
			BeforeEach(func() {
				configFile = "./test_assets/loggregator.json"
			})

			It("returns proper config", func() {
				config, _ := main.ParseConfig(&logLevel, &configFile, &logFilePath)

				Expect(config.IncomingPort).To(Equal(uint32(8765)))
				Expect(config.OutgoingPort).To(Equal(uint32(4567)))
				Expect(config.WSMessageBufferSize).To(Equal(uint(100)))
				Expect(config.BlackListIps[0].Start).To(Equal("127.0.0.0"))
				Expect(config.BlackListIps[0].End).To(Equal("127.0.0.2"))
				Expect(config.BlackListIps[1].Start).To(Equal("127.0.1.12"))
				Expect(config.BlackListIps[1].End).To(Equal("127.0.1.15"))
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
		var oldProvider func([]string, int) storeadapter.StoreAdapter
		BeforeEach(func() {
			adapter = fakestoreadapter.New()
			oldProvider = main.StoreAdapterProvider
			main.StoreAdapterProvider = func([]string, int) storeadapter.StoreAdapter {
				return adapter
			}
		})

		AfterEach(func() {
			main.StoreAdapterProvider = oldProvider
		})

		Context("with a valid ETCD conig", func() {
			var config main.Config

			BeforeEach(func() {
				config = main.Config{
					JobName: "loggregator_z1",
					Index:   0,
					EtcdMaxConcurrentRequests: 10,
					EtcdUrls:                  []string{"test:123", "test:456"},
					Zone:                      "z1",
					IncomingPort:              1234,
				}
			})

			It("connects to etcd", func() {
				main.StartHeartbeats(time.Second, &config, loggertesthelper.Logger())
				Expect(adapter.DidConnect).To(BeTrue())
			})

			It("sends a heartbeat to etcd", func() {
				main.StartHeartbeats(time.Second, &config, loggertesthelper.Logger())
				Expect(adapter.MaintainedNodeName).To(Equal("/healthstatus/loggregator/z1/loggregator_z1/0"))
				local_ip, _ := localip.LocalIP()
				Expect(adapter.MaintainedNodeValue).To(Equal([]byte(local_ip)))
			})

			Context("when there is an error", func() {
				It("panics", func() {
					adapter.MaintainNodeError = errors.New("error")
					Expect(func() { main.StartHeartbeats(time.Second, &config, loggertesthelper.Logger()) }).To(Panic())
				})
			})
		})

		Context("without a valid ETCD config", func() {
			It("does not send a heartbeat", func() {
				config := main.Config{
					JobName: "loggregator_z1",
					Index:   0,
					EtcdMaxConcurrentRequests: 10,
				}

				main.StartHeartbeats(time.Second, &config, loggertesthelper.Logger())
				Expect(adapter.MaintainedNodeName).To(BeEmpty())
			})
		})
	})

})
