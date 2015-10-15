package announcer_test

import (
	"doppler/announcer"
	"doppler/config"
	"errors"
	"fmt"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/storeadapter/fakestoreadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("Announcer", func() {

	var adapter *fakestoreadapter.FakeStoreAdapter

	BeforeEach(func() {
		adapter = fakestoreadapter.New()
	})

	Context("with valid ETCD config", func() {
		var dopplerKey string

		BeforeEach(func() {
			dopplerKey = fmt.Sprintf("/doppler/meta/%s/%s/%d", conf.Zone, conf.JobName, conf.Index)
		})

		It("Panics if etcd returns error", func() {
			err := errors.New("some etcd time out error")
			adapter.MaintainNodeError = err
			Expect(func() {
				announcer.Announce(localIP, time.Second, &conf, adapter, loggertesthelper.Logger())
			}).To(Panic())
		})

		Context("when tls transport is enabled", func() {
			It("has udp and tcp value", func() {
				dopplerMeta := fmt.Sprintf(`{"version": 1, "ip": "%s", "transports":[{"protocol":"udp","port":1234},{"protocol":"tls","port":4567}]}`, localIP)

				conf.EnableTLSTransport = true
				conf.TLSListenerConfig = config.TLSListenerConfig{
					Port: 4567,
				}
				announcer.Announce(localIP, time.Second, &conf, adapter, loggertesthelper.Logger())

				Expect(adapter.GetMaintainedNodeName()).To(Equal(dopplerKey))
				Expect(adapter.MaintainedNodeValue).To(MatchJSON(dopplerMeta))
			})
		})

		Context("when tls transport is disabled", func() {
			It("has only udp value", func() {
				dopplerMeta := fmt.Sprintf(`{"version": 1, "ip":"%s","transports":[{"protocol":"udp","port":1234}]}`, localIP)

				conf.EnableTLSTransport = false
				announcer.Announce(localIP, time.Second, &conf, adapter, loggertesthelper.Logger())

				Expect(adapter.GetMaintainedNodeName()).To(Equal(dopplerKey))
				Expect(adapter.MaintainedNodeValue).To(MatchJSON(dopplerMeta))
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

			announcer.Announce(localIP, time.Second, &conf, adapter, loggertesthelper.Logger())
			Expect(adapter.GetMaintainedNodeName()).To(BeEmpty())
		})
	})

	Context("AnnounceLegacy", func() {
		var legacyKey string
		BeforeEach(func() {
			legacyKey = fmt.Sprintf("/healthstatus/doppler/%s/%s/%d", conf.Zone, conf.JobName, conf.Index)
		})
		It("Should maintain legacy healthstatus key and value", func() {

			announcer.AnnounceLegacy(localIP, time.Second, &conf, adapter, loggertesthelper.Logger())
			Expect(adapter.GetMaintainedNodeName()).To(Equal(legacyKey))
			Expect(string(adapter.MaintainedNodeValue)).To(Equal(localIP))
		})
	})

})
