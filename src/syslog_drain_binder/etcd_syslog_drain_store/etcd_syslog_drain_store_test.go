package etcd_syslog_drain_store_test

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"syslog_drain_binder/etcd_syslog_drain_store"
	"syslog_drain_binder/shared_types"
	"time"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
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
		syslogDrainStore = etcd_syslog_drain_store.NewEtcdSyslogDrainStore(fakeStoreAdapter, 10*time.Second, loggertesthelper.Logger())
	})

	Describe("UpdateDrains", func() {
		It("writes drain urls to the store adapter", func() {
			appDrainUrlMap := map[shared_types.AppId][]shared_types.DrainURL{
				"app-id": {"url1", "url2"},
			}

			err := syslogDrainStore.UpdateDrains(appDrainUrlMap)
			Expect(err).ToNot(HaveOccurred())

			node, err := fakeStoreAdapter.Get(drainKey("app-id", "url1"))
			Expect(err).ToNot(HaveOccurred())
			Expect(node.Value).To(BeEquivalentTo("url1"))

			node, _ = fakeStoreAdapter.Get(drainKey("app-id", "url2"))
			Expect(node.Value).To(BeEquivalentTo("url2"))
		})

		It("sets TTL on the app node if there are drain changes", func() {
			appDrainUrlMap := map[shared_types.AppId][]shared_types.DrainURL{
				"app-id": {"url1"},
			}
			syslogDrainStore.UpdateDrains(appDrainUrlMap)
			node, _ := fakeStoreAdapter.Get(drainKey("app-id", "url1"))
			Expect(node.TTL).To(BeEquivalentTo(10))
		})

		It("returns an error if adapter.SetMulti fails", func() {
			fakeError := errors.New("fake error")
			fakeStoreAdapter.SetErrInjector = fakestoreadapter.NewFakeStoreAdapterErrorInjector(".*", fakeError)
			appDrainUrlMap := map[shared_types.AppId][]shared_types.DrainURL{
				"app-id": {"url1"},
			}
			err := syslogDrainStore.UpdateDrains(appDrainUrlMap)
			Expect(err).To(Equal(fakeError))
		})

		It("does not store drain nodes if they have an empty URL", func() {
			appDrainUrlMap := map[shared_types.AppId][]shared_types.DrainURL{
				"app-id": {" ", "", "\t"},
			}
			syslogDrainStore.UpdateDrains(appDrainUrlMap)
			Expect(fakeStoreAdapter.SetKeyCounters).NotTo(HaveKey(drainKey("app-id", "")))
			Expect(fakeStoreAdapter.SetKeyCounters).NotTo(HaveKey(drainKey("app-id", " ")))
			Expect(fakeStoreAdapter.SetKeyCounters).NotTo(HaveKey(drainKey("app-id", "\t")))
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

func appKey(appId shared_types.AppId) string {
	return fmt.Sprintf("/loggregator/services/%s", appId)
}

func drainKey(appId shared_types.AppId, drainUrl shared_types.DrainURL) string {
	hash := sha1.Sum([]byte(drainUrl))
	return fmt.Sprintf("%s/%x", appKey(appId), hash)
}
