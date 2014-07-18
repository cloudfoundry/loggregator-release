package main_test

import (
	"fmt"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
	"trafficcontroller"
)

var _ = Describe("Etcd Integration tests", func() {
	var config main.Config

	BeforeEach(func() {
		config = main.Config{
			JobName:                   "loggregator_trafficcontroller",
			JobIndex:                  0,
			EtcdMaxConcurrentRequests: 1,
			EtcdUrls:                  []string{fmt.Sprintf("http://127.0.0.1:%d", etcdPort)},
		}

	})

	Describe("Heartbeats", func() {
		It("arrives safely in etcd", func() {
			adapter := etcdRunner.Adapter()

			Consistently(func() error {
				_, err := adapter.Get("healthstatus/loggregator_trafficcontroller/0")
				return err
			}).Should(HaveOccurred())

			main.StartHeartbeats(time.Second, &config, loggertesthelper.Logger())

			Eventually(func() error {
				_, err := adapter.Get("healthstatus/loggregator_trafficcontroller/0")
				return err
			}).ShouldNot(HaveOccurred())
		})

		It("has a 10 sec TTL", func() {
			main.StartHeartbeats(time.Second, &config, loggertesthelper.Logger())
			adapter := etcdRunner.Adapter()

			Eventually(func() uint64 {
				node, _ := adapter.Get("healthstatus/loggregator_trafficcontroller/0")
				return node.TTL
			}).Should(BeNumerically(">", 0))
		})

		It("updates the value periodically", func() {
			main.StartHeartbeats(time.Second, &config, loggertesthelper.Logger())
			adapter := etcdRunner.Adapter()

			var indices []uint64
			var index uint64
			Eventually(func() uint64 {
				node, _ := adapter.Get("healthstatus/loggregator_trafficcontroller/0")
				index = node.Index
				return node.Index
			}).Should(BeNumerically(">", 0))
			indices = append(indices, index)

			for i := 0; i < 3; i++ {
				Eventually(func() uint64 {
					node, _ := adapter.Get("healthstatus/loggregator_trafficcontroller/0")
					index = node.Index
					return node.Index
				}).Should(BeNumerically(">", indices[i]))
				indices = append(indices, index)

			}
		})
	})
})
