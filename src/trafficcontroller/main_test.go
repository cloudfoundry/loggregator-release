package main_test

import (
	"errors"
	"time"
	"trafficcontroller"

	"github.com/cloudfoundry/loggregatorlib/cfcomponent/localip"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/fakestoreadapter"

	"github.com/cloudfoundry/gosteno"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("HashingEnabled", func() {
	var logger = gosteno.NewLogger("TestLogger")
	It("returns true if the loggregators are configured", func() {
		config := &main.Config{
			Loggregators: map[string][]string{
				"z1": []string{"10.244.0.14"},
				"z2": []string{},
			},
		}
		Expect(main.HashingEnabled(config, logger)).To(BeTrue())
	})

	It("returns false if the loggregators are not configured", func() {
		config := &main.Config{}
		Expect(main.HashingEnabled(config, logger)).To(BeFalse())
	})
})

var _ = Describe("Main", func() {
	Describe("MakeHashers", func() {
		Context("with an empty AZ", func() {
			It("ignores the empty AZ and returns a single hasher", func() {
				config := &main.Config{
					Loggregators: map[string][]string{
						"z1": []string{"10.244.0.14"},
						"z2": []string{},
					},
				}

				hashers := main.MakeHashers(config.Loggregators, 3456, loggertesthelper.Logger())
				Expect(hashers).To(HaveLen(1))
			})

		})

		Context("with two AZs", func() {
			It("returns two hashers", func() {
				config := &main.Config{
					Loggregators: map[string][]string{
						"z1": []string{"10.244.0.14"},
						"z2": []string{"10.244.0.14"},
					},
				}

				hashers := main.MakeHashers(config.Loggregators, 3456, loggertesthelper.Logger())
				Expect(hashers).To(HaveLen(2))
			})
		})
	})

	Describe("ParseConfig", func() {
		var (
			logLevel    = false
			logFilePath = "./test_assets/stdout.log"
		)

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
				configFile := "./test_assets/no_hashing_loggregator_trafficcontroller.json"

				var config *main.Config

				config, _, _ = main.ParseConfig(&logLevel, &configFile, &logFilePath)

				Expect(config.JobName).To(Equal("trafficcontroller"))
				Expect(config.JobIndex).To(Equal(3))
				Expect(config.EtcdMaxConcurrentRequests).To(Equal(5))
				Expect(config.EtcdUrls).To(ConsistOf([]string{"http://127.0.0.1:4001", "http://127.0.0.1:4002"}))
			})
		})
	})

	Describe("StartHeartbeats", func() {
		var adapter *fakestoreadapter.FakeStoreAdapter
		var oldProvider func([]string, int) storeadapter.StoreAdapter
		BeforeEach(func() {
			adapter = fakestoreadapter.New()
			oldProvider = main.DefaultStoreAdapterProvider
			main.DefaultStoreAdapterProvider = func([]string, int) storeadapter.StoreAdapter {
				return adapter
			}
		})

		AfterEach(func() {
			main.DefaultStoreAdapterProvider = oldProvider
		})

		Context("with a valid ETCD conig", func() {
			var config main.Config

			BeforeEach(func() {
				config = main.Config{
					JobName:                   "loggregator_trafficcontroller",
					JobIndex:                  0,
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
				Expect(adapter.MaintainedNodeName).To(Equal("/healthstatus/trafficcontroller/z1/loggregator_trafficcontroller/0"))
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
					JobName:                   "loggregator_trafficcontroller",
					JobIndex:                  0,
					EtcdMaxConcurrentRequests: 10,
				}

				main.StartHeartbeats(time.Second, &config, loggertesthelper.Logger())
				Expect(adapter.MaintainedNodeName).To(BeEmpty())
			})
		})
	})
})
