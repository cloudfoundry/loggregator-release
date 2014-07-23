package main_test

import (
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/storeadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
	"trafficcontroller"
)

var _ = Describe("Provider", func() {
	var config main.Config
	var logger = gosteno.NewLogger("TestLogger")

	Context("with hasher configuration", func() {
		BeforeEach(func() {
			config = main.Config{
				JobName:                 "loggregator_trafficcontroller",
				JobIndex:                0,
				Zone:                    "z1",
				Loggregators:            map[string][]string{"z1": {"1.2.3.4", "1.2.3.5"}, "z2": {"5.6.7.8", "5.6.7.9"}},
				LoggregatorOutgoingPort: 3456,
			}
		})
		It("returns a hashing provider", func() {
			provider := main.MakeProvider(&config, logger, nil)
			servers := provider.LoggregatorServersForAppId("123")
			Expect(servers).To(ConsistOf("1.2.3.5:3456", "5.6.7.9:3456"))
		})
	})
	Context("with dynamic configuration", func() {
		BeforeEach(func() {
			main.EtcdQueryInterval = 10 * time.Millisecond
			config = main.Config{
				JobName:  "loggregator_trafficcontroller",
				JobIndex: 0,
				Zone:     "z1",
				EtcdMaxConcurrentRequests: 1,
				EtcdUrls:                  []string{fmt.Sprintf("http://127.0.0.1:%d", etcdPort)},
				LoggregatorOutgoingPort:   3456,
			}
			node := storeadapter.StoreNode{
				Key:   "healthstatus/loggregator/z1/loggregator/0",
				Value: []byte("1.2.3.4"),
			}
			adapter := etcdRunner.Adapter()
			adapter.Create(node)
		})
		It("returns a hashing provider", func() {
			stopChan := make(chan struct{})
			defer close(stopChan)
			provider := main.MakeProvider(&config, logger, stopChan)

			Eventually(func() []string {
				return provider.LoggregatorServersForAppId("123")
			}).Should(ConsistOf("1.2.3.4:3456"))

		})
	})
})

var _ = Describe("Etcd Integration tests", func() {
	var config main.Config

	BeforeEach(func() {
		config = main.Config{
			JobName:                   "loggregator_trafficcontroller",
			JobIndex:                  0,
			EtcdMaxConcurrentRequests: 1,
			EtcdUrls:                  []string{fmt.Sprintf("http://127.0.0.1:%d", etcdPort)},
			Zone:                      "z1",
		}

	})

	Describe("Heartbeats", func() {
		It("arrives safely in etcd", func() {
			adapter := etcdRunner.Adapter()

			Consistently(func() error {
				_, err := adapter.Get("healthstatus/trafficcontroller/z1/loggregator_trafficcontroller/0")
				return err
			}).Should(HaveOccurred())

			main.StartHeartbeats(time.Second, &config, loggertesthelper.Logger())

			Eventually(func() error {
				_, err := adapter.Get("healthstatus/trafficcontroller/z1/loggregator_trafficcontroller/0")
				return err
			}).ShouldNot(HaveOccurred())
		})

		It("has a 10 sec TTL", func() {
			main.StartHeartbeats(time.Second, &config, loggertesthelper.Logger())
			adapter := etcdRunner.Adapter()

			Eventually(func() uint64 {
				node, _ := adapter.Get("healthstatus/trafficcontroller/z1/loggregator_trafficcontroller/0")
				return node.TTL
			}).Should(BeNumerically(">", 0))
		})

		It("updates the value periodically", func() {
			main.StartHeartbeats(time.Second, &config, loggertesthelper.Logger())
			adapter := etcdRunner.Adapter()

			var indices []uint64
			var index uint64
			Eventually(func() uint64 {
				node, _ := adapter.Get("healthstatus/trafficcontroller/z1/loggregator_trafficcontroller/0")
				index = node.Index
				return node.Index
			}).Should(BeNumerically(">", 0))
			indices = append(indices, index)

			for i := 0; i < 3; i++ {
				Eventually(func() uint64 {
					node, _ := adapter.Get("healthstatus/trafficcontroller/z1/loggregator_trafficcontroller/0")
					index = node.Index
					return node.Index
				}).Should(BeNumerically(">", indices[i]))
				indices = append(indices, index)

			}
		})
	})
})
