package syslog_drain_store_test

import (
	"deaagent/syslog_drain_store"
	"errors"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/fakestoreadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SyslogDrainStore", func() {
	var (
		fakeStoreAdapter *FakeStoreAdapter
		syslogDrainStore syslog_drain_store.SyslogDrainStore
	)

	BeforeEach(func() {
		fakeStoreAdapter = NewFakeStoreAdapter()
		syslogDrainStore = syslog_drain_store.NewSyslogDrainStore(fakeStoreAdapter, loggertesthelper.Logger())
	})

	Describe("AppKey", func() {
		It("returns the right value", func() {
			result := syslog_drain_store.AppKey("app-id")
			Expect(result).To(Equal("/loggregator/services/app-id"))
		})
	})

	Describe("DrainKey", func() {
		It("returns the right value", func() {
			result := syslog_drain_store.DrainKey("app-id", "drain-url")
			Expect(result).To(Equal("/loggregator/services/app-id/b46efa803d8374840d2c2f7ec69648097202c3f2"))
		})
	})

	Describe("UpdateDrains", func() {
		It("writes drain urls to the store adapter", func() {
			drainUrls := []string{"url1", "url2"}
			err := syslogDrainStore.UpdateDrains("app-id", drainUrls)
			Expect(err).ToNot(HaveOccurred())

			node, err := fakeStoreAdapter.Get(syslog_drain_store.DrainKey("app-id", "url1"))
			Expect(err).ToNot(HaveOccurred())
			Expect(node.Value).To(BeEquivalentTo("url1"))

			node, _ = fakeStoreAdapter.Get(syslog_drain_store.DrainKey("app-id", "url2"))
			Expect(node.Value).To(BeEquivalentTo("url2"))
		})

		It("sets TTL on the app node if there are drain changes", func() {
			drainUrls := []string{"url1"}
			syslogDrainStore.UpdateDrains("app-id", drainUrls)

			Expect(fakeStoreAdapter.UpdateDirTTL_lastKey).To(Equal(syslog_drain_store.AppKey("app-id")))
			Expect(fakeStoreAdapter.UpdateDirTTL_lastTtl).To(BeEquivalentTo(syslog_drain_store.APP_NODE_TTL.Seconds()))
		})

		It("does not set TTL on the app node if there are no drain changes", func() {
			drainUrls := []string{"url1"}
			syslogDrainStore.UpdateDrains("app-id", drainUrls)

			fakeStoreAdapter.UpdateDirTTL_lastKey = ""
			fakeStoreAdapter.UpdateDirTTL_lastTtl = 0

			syslogDrainStore.UpdateDrains("app-id", drainUrls)

			Expect(fakeStoreAdapter.UpdateDirTTL_lastKey).To(Equal(""))
			Expect(fakeStoreAdapter.UpdateDirTTL_lastTtl).To(BeEquivalentTo(0))
		})

		It("removes old drain urls", func() {
			syslogDrainStore.UpdateDrains("app-id", []string{"url1", "url2"})

			_, err := fakeStoreAdapter.Get(syslog_drain_store.DrainKey("app-id", "url2"))
			Expect(err).ToNot(HaveOccurred())

			syslogDrainStore.UpdateDrains("app-id", []string{"url1"})

			_, err = fakeStoreAdapter.Get(syslog_drain_store.DrainKey("app-id", "url2"))
			Expect(err).To(HaveOccurred())
		})

		It("does not set drain nodes if they already exist", func() {
			syslogDrainStore.UpdateDrains("app-id", []string{"url1"})
			syslogDrainStore.UpdateDrains("app-id", []string{"url1"})
			Expect(fakeStoreAdapter.SetKeyCounters).To(HaveKeyWithValue(syslog_drain_store.DrainKey("app-id", "url1"), 1))
		})

		It("returns an error if adapter.ListRecursively returns an error other than 'key not found'", func() {
			fakeError := errors.New("fake error")
			fakeStoreAdapter.ListErrInjector = fakestoreadapter.NewFakeStoreAdapterErrorInjector(".*", fakeError)
			err := syslogDrainStore.UpdateDrains("app-id", []string{})
			Expect(err).To(Equal(fakeError))
		})

		It("returns an error if adapter.Delete fails", func() {
			fakeError := errors.New("fake error")
			fakeStoreAdapter.DeleteErrInjector = fakestoreadapter.NewFakeStoreAdapterErrorInjector(".*", fakeError)
			syslogDrainStore.UpdateDrains("app-id", []string{"url1"})
			err := syslogDrainStore.UpdateDrains("app-id", []string{})
			Expect(err).To(Equal(fakeError))
		})

		It("returns an error if adapter.SetMulti fails", func() {
			fakeError := errors.New("fake error")
			fakeStoreAdapter.SetErrInjector = fakestoreadapter.NewFakeStoreAdapterErrorInjector(".*", fakeError)
			err := syslogDrainStore.UpdateDrains("app-id", []string{"url1"})
			Expect(err).To(Equal(fakeError))
		})

		It("returns an error if adapter.UpdateDirTTL fails", func() {
			fakeError := errors.New("fake error")
			fakeStoreAdapter.UpdateDirTTL_error = fakeError
			err := syslogDrainStore.UpdateDrains("app-id", []string{"url1"})
			Expect(err).To(Equal(fakeError))
		})

		It("does not store drain nodes if they have an empty URL", func() {
			syslogDrainStore.UpdateDrains("app-id", []string{" ", "", "\t"})
			Expect(fakeStoreAdapter.SetKeyCounters).NotTo(HaveKey(syslog_drain_store.DrainKey("app-id", "")))
			Expect(fakeStoreAdapter.SetKeyCounters).NotTo(HaveKey(syslog_drain_store.DrainKey("app-id", " ")))
			Expect(fakeStoreAdapter.SetKeyCounters).NotTo(HaveKey(syslog_drain_store.DrainKey("app-id", "\t")))
		})
	})

	var _ = Describe("RefreshAppNode", func() {
		It("updates the app node's TTL", func() {
			fakeStoreAdapter.SetMulti([]storeadapter.StoreNode{{
				Key:   syslog_drain_store.DrainKey("app-id", "url"),
				Value: []byte("url"),
			}})

			syslogDrainStore.RefreshAppNode("app-id")

			appKey := syslog_drain_store.AppKey("app-id")
			Expect(fakeStoreAdapter.UpdateDirTTL_lastKey).To(Equal(appKey))
			Expect(fakeStoreAdapter.UpdateDirTTL_lastTtl).To(BeEquivalentTo(syslog_drain_store.APP_NODE_TTL.Seconds()))
		})

		It("returns an error if adapter.UpdateDirTTL fails", func() {
			fakeError := errors.New("fake error")
			fakeStoreAdapter.UpdateDirTTL_error = fakeError
			err := syslogDrainStore.RefreshAppNode("app-id")
			Expect(err).To(Equal(fakeError))
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
