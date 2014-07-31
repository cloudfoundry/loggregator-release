package main_test

import (
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/storeadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
	"trafficcontroller"
)

var _ = Describe("Provider", func() {
	var config main.Config
	var logger = gosteno.NewLogger("TestLogger")

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

		It("returns a dynamic server provider", func() {
			stopChan := make(chan struct{})
			defer close(stopChan)
			provider := main.MakeProvider(&config, logger, stopChan)

			Eventually(func() []string {
				return provider.LoggregatorServerAddresses()
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
})
