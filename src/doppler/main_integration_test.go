package main_test

import (
	"fmt"
	"time"

	"doppler"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"

	"doppler/config"

	"doppler/announcer"

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

const legacyETCDKey = "/healthstatus/doppler/z1/doppler_z1/0"
const dopplerETCDKey = "/doppler/meta/z1/doppler_z1/0"

var _ = Describe("Etcd Integration tests", func() {
	var (
		conf    config.Config
		localIp string
	)
	BeforeEach(func() {
		localIp, _ = localip.LocalIP()

		conf = config.Config{
			JobName: "doppler_z1",
			Index:   0,
			EtcdMaxConcurrentRequests: 1,
			EtcdUrls:                  []string{fmt.Sprintf("http://127.0.0.1:%d", etcdPort)},
			Zone:                      "z1",
			ContainerMetricTTLSeconds:     120,
			DropsondeIncomingMessagesPort: 1234,
		}
	})

	Context("Heartbeats", func() {
		It("arrives safely in etcd", func() {
			storeAdapter := setupAdapter(legacyETCDKey, conf)

			announcer.AnnounceLegacy(localIp, time.Second, &conf, storeAdapter, loggertesthelper.Logger())

			Eventually(func() error {
				_, err := storeAdapter.Get(legacyETCDKey)
				return err
			}, 3).ShouldNot(HaveOccurred())
		})
	})

	Context("Store Transport", func() {
		Context("With EnableTlsTransport set to true", func() {
			It("stores udp and tcp transport in etcd", func() {
				conf.EnableTLSTransport = true
				conf.TLSListenerConfig = config.TLSListenerConfig{
					Port: 4567,
				}
				storeAdapter := setupAdapter(dopplerETCDKey, conf)

				announcer.Announce(localIp, time.Second, &conf, storeAdapter, loggertesthelper.Logger())

				dopplerMeta := createDopplerMeta(localIp, conf)

				Eventually(func() []byte {
					node, _ := storeAdapter.Get(dopplerETCDKey)
					return node.Value
				}, 3).Should(MatchJSON(dopplerMeta))
			})
		})

		Context("With EnableTlsTransport set to false", func() {
			It("only stores udp transport in etcd", func() {
				dopplerMeta := createDopplerMetaNoTLS(localIp, conf)
				conf.EnableTLSTransport = false
				storeAdapter := setupAdapter(dopplerETCDKey, conf)

				announcer.Announce(localIp, time.Second, &conf, storeAdapter, loggertesthelper.Logger())

				Eventually(func() []byte {
					node, _ := storeAdapter.Get(dopplerETCDKey)
					return node.Value
				}, 3).Should(MatchJSON(dopplerMeta))
			})
		})

		It("updates key if it already exists in etcd", func() {
			storeAdapter := setupAdapter(dopplerETCDKey, conf)

			conf.EnableTLSTransport = true
			conf.TLSListenerConfig = config.TLSListenerConfig{
				Port: 4567,
			}
			releaseNodeChan := announcer.Announce(localIp, time.Second, &conf, storeAdapter, loggertesthelper.Logger())
			dopplerMeta := createDopplerMeta(localIp, conf)

			Eventually(func() []byte {
				node, _ := storeAdapter.Get(dopplerETCDKey)

				return node.Value
			}, 3).Should(MatchJSON(dopplerMeta))

			close(releaseNodeChan)

			conf.EnableTLSTransport = false
			releaseNodeChan = announcer.Announce(localIp, time.Second, &conf, storeAdapter, loggertesthelper.Logger())
			updatedDopplerMeta := createDopplerMetaNoTLS(localIp, conf)

			Eventually(func() []byte {
				node, _ := storeAdapter.Get(dopplerETCDKey)
				return node.Value
			}, 3).Should(MatchJSON(updatedDopplerMeta))

			close(releaseNodeChan)
		})

		It("announces legacy healthstatus and latest metada in etcd", func() {

			conf.EnableTLSTransport = true
			conf.TLSListenerConfig = config.TLSListenerConfig{
				Port: 4567,
			}
			dopplerMeta := createDopplerMeta(localIp, conf)
			storeAdapter := setupAdapter(dopplerETCDKey, conf)

			metaReleaseChan := announcer.Announce(localIp, time.Second, &conf, storeAdapter, loggertesthelper.Logger())

			Eventually(func() []byte {
				node, _ := storeAdapter.Get(dopplerETCDKey)
				return node.Value
			}, 3).Should(MatchJSON(dopplerMeta))
			close(metaReleaseChan)

			legacyStoreAdapter := setupAdapter(legacyETCDKey, conf)
			legacyReleaseChan := announcer.AnnounceLegacy(localIp, time.Second, &conf, legacyStoreAdapter, loggertesthelper.Logger())

			Eventually(func() string {
				node, _ := storeAdapter.Get(legacyETCDKey)
				return string(node.Value)
			}, 3).Should(Equal(localIp))
			close(legacyReleaseChan)
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

func createDopplerMeta(localIp string, conf config.Config) string {
	dopplerMeta := fmt.Sprintf(`{"version":1,"ip":"%s","transports":[{"protocol":"udp","port":%d},{"protocol":"tls","port":%d}]}`, localIp, conf.DropsondeIncomingMessagesPort, conf.TLSListenerConfig.Port)
	return dopplerMeta
}

func createDopplerMetaNoTLS(localIp string, conf config.Config) string {
	dopplerMeta := fmt.Sprintf(`{"version":1,"ip":"%s","transports":[{"protocol":"udp","port":%d}]}`, localIp, conf.DropsondeIncomingMessagesPort)
	return dopplerMeta
}
