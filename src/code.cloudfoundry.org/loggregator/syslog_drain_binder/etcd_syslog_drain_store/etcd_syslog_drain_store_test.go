package etcd_syslog_drain_store_test

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"time"

	"code.cloudfoundry.org/loggregator/syslog_drain_binder/etcd_syslog_drain_store"
	"code.cloudfoundry.org/loggregator/syslog_drain_binder/shared_types"

	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/fakestoreadapter"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EtcdSyslogDrainStore", func() {
	var (
		fakeStoreAdapter *FakeStoreAdapter
		syslogDrainStore *etcd_syslog_drain_store.EtcdSyslogDrainStore
	)

	BeforeEach(func() {
		fakeStoreAdapter = NewFakeStoreAdapter()
		syslogDrainStore = etcd_syslog_drain_store.NewEtcdSyslogDrainStore(fakeStoreAdapter, 10*time.Second)
	})

	Describe("UpdateDrains", func() {
		It("writes drain urls to the store adapter", func() {
			appDrainUrlMap := shared_types.AllSyslogDrainBindings{
				"app-id": shared_types.SyslogDrainBinding{DrainURLs: []string{"url1", "url2"}, Hostname: "org.space.app.1"},
			}

			err := syslogDrainStore.UpdateDrains(appDrainUrlMap)
			Expect(err).ToNot(HaveOccurred())
			drainData := `{"hostname":"org.space.app.1","drainURL":"url1"}`
			node, err := fakeStoreAdapter.Get(drainKey("app-id", drainData))
			Expect(err).ToNot(HaveOccurred())
			Expect(node.Value).To(MatchJSON(drainData))

			drainData = `{"hostname":"org.space.app.1","drainURL":"url2"}`
			node, _ = fakeStoreAdapter.Get(drainKey("app-id", drainData))
			Expect(node.Value).To(MatchJSON(drainData))
		})

		It("sets TTL on the app node if there are drain changes", func() {
			appDrainUrlMap := shared_types.AllSyslogDrainBindings{
				"app-id": shared_types.SyslogDrainBinding{DrainURLs: []string{"url1"}, Hostname: "org.space.app.1"},
			}
			drainData := `{"hostname":"org.space.app.1","drainURL":"url1"}`

			syslogDrainStore.UpdateDrains(appDrainUrlMap)

			node, _ := fakeStoreAdapter.Get(drainKey("app-id", drainData))
			Expect(node.TTL).To(BeEquivalentTo(10))
		})

		It("returns an error if adapter.SetMulti fails", func() {
			fakeError := errors.New("fake error")
			fakeStoreAdapter.SetErrInjector = fakestoreadapter.NewFakeStoreAdapterErrorInjector(".*", fakeError)
			appDrainUrlMap := shared_types.AllSyslogDrainBindings{
				"app-id": shared_types.SyslogDrainBinding{DrainURLs: []string{"url1"}, Hostname: "org.space.app.1"},
			}

			err := syslogDrainStore.UpdateDrains(appDrainUrlMap)
			Expect(err).To(Equal(fakeError))
		})

		It("does not store drain nodes if they have an empty URL", func() {
			appDrainUrlMap := shared_types.AllSyslogDrainBindings{
				"app-id": shared_types.SyslogDrainBinding{DrainURLs: []string{" ", "\t "}, Hostname: "org.space.app.1"},
			}
			syslogDrainStore.UpdateDrains(appDrainUrlMap)
			Expect(fakeStoreAdapter.SetKeyCounters).To(HaveLen(0))
		})
	})
})

type FakeStoreAdapter struct {
	*fakestoreadapter.FakeStoreAdapter
	UpdateDirTTL_lastKey string
	UpdateDirTTL_lastTtl uint64
	UpdateDirTTL_error   error
	SetKeyCounters       map[string]int
}

func NewFakeStoreAdapter() *FakeStoreAdapter {
	return &FakeStoreAdapter{
		FakeStoreAdapter: fakestoreadapter.New(),
		SetKeyCounters:   make(map[string]int),
	}
}

func (adapter *FakeStoreAdapter) UpdateDirTTL(key string, ttl uint64) error {
	adapter.UpdateDirTTL_lastKey = key
	adapter.UpdateDirTTL_lastTtl = ttl
	return adapter.UpdateDirTTL_error
}

func (adapter *FakeStoreAdapter) SetMulti(nodes []storeadapter.StoreNode) error {
	for _, node := range nodes {
		adapter.SetKeyCounters[string(node.Key)] += 1
	}
	return adapter.FakeStoreAdapter.SetMulti(nodes)
}

func appKey(appId shared_types.AppID) string {
	return fmt.Sprintf("/loggregator/v2/services/%s", appId)
}

func drainKey(appId shared_types.AppID, drainBinding string) string {
	hash := sha1.Sum([]byte(drainBinding))
	return fmt.Sprintf("%s/%x", appKey(appId), hash)
}
