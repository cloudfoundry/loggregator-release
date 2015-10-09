package main_test

import (
	"fmt"
	"time"

	"doppler"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"

	"doppler/config"

	"github.com/cloudfoundry/storeadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/localip"
)

var _ = BeforeEach(func() {
	adapter := etcdRunner.Adapter()
	adapter.Disconnect()
	etcdRunner.Reset()
	adapter.Connect()
})

var _ = Describe("Etcd Integration tests", func() {
	var conf config.Config
	var stopHeartbeats chan (chan bool)
	var localIp string

	BeforeEach(func() {
		stopHeartbeats = nil

		localIp, _ = localip.LocalIP()

		conf = config.Config{
			JobName: "doppler_z1",
			Index:   0,
			EtcdMaxConcurrentRequests: 1,
			EtcdUrls:                  []string{fmt.Sprintf("http://127.0.0.1:%d", etcdPort)},
			Zone:                      "z1",
			ContainerMetricTTLSeconds: 120,
		}
	})

	Describe("Heartbeats", func() {
		AfterEach(func() {
			if stopHeartbeats != nil {
				heartbeatsStopped := make(chan bool)
				stopHeartbeats <- heartbeatsStopped
				<-heartbeatsStopped
			}
		})

		It("arrives safely in etcd", func() {
			storeAdapter := setupAdapter("healthstatus/doppler/z1/doppler_z1/0", conf)

			stopHeartbeats = main.StartHeartbeats(localIp, time.Second, &conf, storeAdapter, loggertesthelper.Logger())

			Eventually(func() error {
				_, err := storeAdapter.Get("healthstatus/doppler/z1/doppler_z1/0")
				return err
			}, 3).ShouldNot(HaveOccurred())
		})
	})

	Describe("Store Transport", func() {
		Context("With EnableTlsTransport set to true", func() {
			It("stores udp and tcp transport in etcd", func() {
				conf.EnableTLSTransport = true
				storeAdapter := setupAdapter("/doppler/dropsonde-transport/z1/doppler_z1/0", conf)

				main.StoreTransport(&conf, storeAdapter, loggertesthelper.Logger())

				Eventually(func() []byte {
					node, err := storeAdapter.Get("/doppler/dropsonde-transport/z1/doppler_z1/0")
					Expect(err).ToNot(HaveOccurred())
					return node.Value
				}, 3).Should(Equal([]byte("udp,tcp")))
			})
		})

		Context("With EnableTlsTransport set to false", func() {
			It("only stores udp transport in etcd", func() {

				conf.EnableTLSTransport = false
				storeAdapter := setupAdapter("/doppler/dropsonde-transport/z1/doppler_z1/0", conf)

				main.StoreTransport(&conf, storeAdapter, loggertesthelper.Logger())

				Eventually(func() []byte {
					node, err := storeAdapter.Get("/doppler/dropsonde-transport/z1/doppler_z1/0")
					Expect(err).ToNot(HaveOccurred())
					return node.Value
				}, 3).Should(Equal([]byte("udp")))
			})
		})

		It("updates key if it already exists in etcd", func() {
			conf.EnableTLSTransport = true
			storeAdapter := setupAdapter("/doppler/dropsonde-transport/z1/doppler_z1/0", conf)

			main.StoreTransport(&conf, storeAdapter, loggertesthelper.Logger())

			Eventually(func() []byte {
				node, err := storeAdapter.Get("/doppler/dropsonde-transport/z1/doppler_z1/0")
				Expect(err).ToNot(HaveOccurred())
				return node.Value
			}, 3).Should(Equal([]byte("udp,tcp")))

			conf.EnableTLSTransport = false

			main.StoreTransport(&conf, storeAdapter, loggertesthelper.Logger())

			Eventually(func() []byte {
				node, err := storeAdapter.Get("/doppler/dropsonde-transport/z1/doppler_z1/0")
				Expect(err).ToNot(HaveOccurred())
				return node.Value
			}, 3).Should(Equal([]byte("udp")))
		})

	})
})

func setupAdapter(key string, conf config.Config) storeadapter.StoreAdapter {

	adapter := etcdRunner.Adapter()

	Consistently(func() error {
		_, err := adapter.Get(key)
		return err
	}).Should(HaveOccurred())

	return main.NewStoreAdapter(conf.EtcdUrls, conf.EtcdMaxConcurrentRequests)
}
