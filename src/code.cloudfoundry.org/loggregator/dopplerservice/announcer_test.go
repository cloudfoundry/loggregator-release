package dopplerservice_test

import (
	"errors"
	"fmt"
	"time"

	"code.cloudfoundry.org/loggregator/doppler/app"
	"code.cloudfoundry.org/loggregator/dopplerservice"
	"code.cloudfoundry.org/loggregator/dopplerservice/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	ginkgoConfig "github.com/onsi/ginkgo/config"
)

// Using etcd for service discovery is a deprecated code path
var _ = XDescribe("Announcer", func() {
	var (
		ip          string
		conf        app.Config
		etcdRunner  *etcdstorerunner.ETCDClusterRunner
		etcdAdapter storeadapter.StoreAdapter
	)

	BeforeSuite(func() {
		ip = "127.0.0.1"

		etcdPort := 5500 + ginkgoConfig.GinkgoConfig.ParallelNode*10
		etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1, nil)
		etcdRunner.Start()

		etcdAdapter = etcdRunner.Adapter(nil)

		conf = app.Config{
			JobName: "doppler_z1",
			Index:   "0",
			EtcdMaxConcurrentRequests: 10,
			EtcdUrls:                  etcdRunner.NodeURLS(),
			Zone:                      "z1",
			IncomingUDPPort:           1234,
			OutgoingPort:              8888,
		}
	})

	AfterSuite(func() {
		etcdAdapter.Disconnect()
		etcdRunner.Stop()
	})
	var stopChan chan chan bool

	BeforeEach(func() {
		etcdRunner.Reset()
		stopChan = nil
	})

	AfterEach(func() {
		if stopChan != nil {
			notify := make(chan bool)
			Eventually(stopChan).Should(BeSent(notify))
			Eventually(notify).Should(BeClosed())
		}
	})

	Context("Announce", func() {
		Context("with valid ETCD config", func() {
			var dopplerKey string

			BeforeEach(func() {
				dopplerKey = fmt.Sprintf("/doppler/meta/%s/%s/%s", conf.Zone, conf.JobName, conf.Index)
			})

			It("creates, then maintains the node", func() {
				fakeadapter := &fakes.FakeStoreAdapter{}
				dopplerservice.Announce(ip, time.Second, &conf, fakeadapter)
				Expect(fakeadapter.CreateCallCount()).To(Equal(1))
				Expect(fakeadapter.MaintainNodeCallCount()).To(Equal(1))
			})

			It("Panics if MaintainNode returns error", func() {
				err := errors.New("some etcd time out error")
				fakeadapter := &fakes.FakeStoreAdapter{}
				fakeadapter.MaintainNodeReturns(nil, nil, err)
				Expect(func() {
					dopplerservice.Announce(ip, time.Second, &conf, fakeadapter)
				}).To(Panic())
			})

			It("announces only udp and websocket", func() {
				dopplerMeta := fmt.Sprintf(`{"version": 1, "endpoints":["udp://%[1]s:1234", "ws://%[1]s:8888" ]}`, ip)

				stopChan = dopplerservice.Announce(ip, time.Second, &conf, etcdAdapter)

				Eventually(func() []byte {
					node, err := etcdAdapter.Get(dopplerKey)
					if err != nil {
						return nil
					}
					return node.Value
				}).Should(MatchJSON(dopplerMeta))
			})
		})
	})

	Context("AnnounceLegacy", func() {
		var legacyKey string

		BeforeEach(func() {
			legacyKey = fmt.Sprintf("/healthstatus/doppler/%s/%s/%s", conf.Zone, conf.JobName, conf.Index)
		})

		It("maintains the node", func() {
			fakeadapter := &fakes.FakeStoreAdapter{}
			dopplerservice.AnnounceLegacy(ip, time.Second, &conf, fakeadapter)
			Expect(fakeadapter.MaintainNodeCallCount()).To(Equal(1))
		})

		It("Panics if MaintainNode returns error", func() {
			err := errors.New("some etcd time out error")
			fakeadapter := &fakes.FakeStoreAdapter{}
			fakeadapter.MaintainNodeReturns(nil, nil, err)
			Expect(func() {
				dopplerservice.AnnounceLegacy(ip, time.Second, &conf, fakeadapter)
			}).To(Panic())
		})

		It("Should maintain legacy healthstatus key and value", func() {
			stopChan = dopplerservice.AnnounceLegacy(ip, time.Second, &conf, etcdAdapter)
			Eventually(func() []byte {
				node, err := etcdAdapter.Get(legacyKey)
				if err != nil {
					return nil
				}
				return node.Value
			}).Should(Equal([]byte(ip)))
		})
	})
})
