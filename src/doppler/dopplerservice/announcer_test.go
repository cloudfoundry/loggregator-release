package dopplerservice_test

import (
	"doppler/config"
	"doppler/dopplerservice"
	"doppler/dopplerservice/fakes"
	"errors"
	"fmt"
	"time"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Announcer", func() {
	var stopChan chan chan bool

	BeforeEach(func() {
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
				dopplerKey = fmt.Sprintf("/doppler/meta/%s/%s/%d", conf.Zone, conf.JobName, conf.Index)
			})

			It("maintains the node", func() {
				fakeadapter := &fakes.FakeStoreAdapter{}
				dopplerservice.Announce(localIP, time.Second, &conf, fakeadapter, loggertesthelper.Logger())
				Expect(fakeadapter.MaintainNodeCallCount()).To(Equal(1))
			})

			It("Panics if MaintainNode returns error", func() {
				err := errors.New("some etcd time out error")
				fakeadapter := &fakes.FakeStoreAdapter{}
				fakeadapter.MaintainNodeReturns(nil, nil, err)
				Expect(func() {
					dopplerservice.Announce(localIP, time.Second, &conf, fakeadapter, loggertesthelper.Logger())
				}).To(Panic())
			})

			Context("when tls transport is enabled", func() {
				It("announces udp, tcp, and tls values", func() {
					dopplerMeta := fmt.Sprintf(`{"version": 1, "endpoints":["udp://%s:1234", "tcp://%s:5678", "tls://%s:9012"]}`, localIP, localIP, localIP)

					conf.EnableTLSTransport = true
					conf.TLSListenerConfig = config.TLSListenerConfig{
						Port: 9012,
					}
					stopChan = dopplerservice.Announce(localIP, time.Second, &conf, etcdAdapter, loggertesthelper.Logger())

					Eventually(func() []byte {
						node, err := etcdAdapter.Get(dopplerKey)
						if err != nil {
							return nil
						}
						return node.Value
					}).Should(MatchJSON(dopplerMeta))
				})
			})

			Context("when tls transport is disabled", func() {
				It("announces only udp and tcp values", func() {
					dopplerMeta := fmt.Sprintf(`{"version": 1, "endpoints":["udp://%s:1234", "tcp://%s:5678"]}`, localIP, localIP)

					conf.EnableTLSTransport = false
					stopChan = dopplerservice.Announce(localIP, time.Second, &conf, etcdAdapter, loggertesthelper.Logger())

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
	})

	Context("AnnounceLegacy", func() {
		var legacyKey string

		BeforeEach(func() {
			legacyKey = fmt.Sprintf("/healthstatus/doppler/%s/%s/%d", conf.Zone, conf.JobName, conf.Index)
		})

		It("maintains the node", func() {
			fakeadapter := &fakes.FakeStoreAdapter{}
			dopplerservice.AnnounceLegacy(localIP, time.Second, &conf, fakeadapter, loggertesthelper.Logger())
			Expect(fakeadapter.MaintainNodeCallCount()).To(Equal(1))
		})

		It("Panics if MaintainNode returns error", func() {
			err := errors.New("some etcd time out error")
			fakeadapter := &fakes.FakeStoreAdapter{}
			fakeadapter.MaintainNodeReturns(nil, nil, err)
			Expect(func() {
				dopplerservice.AnnounceLegacy(localIP, time.Second, &conf, fakeadapter, loggertesthelper.Logger())
			}).To(Panic())
		})

		It("Should maintain legacy healthstatus key and value", func() {
			stopChan = dopplerservice.AnnounceLegacy(localIP, time.Second, &conf, etcdAdapter, loggertesthelper.Logger())
			Eventually(func() []byte {
				node, err := etcdAdapter.Get(legacyKey)
				if err != nil {
					return nil
				}
				return node.Value
			}).Should(Equal([]byte(localIP)))
		})
	})
})
