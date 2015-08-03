package main_test

import (
	"trafficcontroller"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/fakestoreadapter"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MakeProvider", func() {
	var (
		originalEtcdQueryInterval time.Duration
		fakeStoreAdapter          *fakestoreadapter.FakeStoreAdapter
	)

	BeforeEach(func() {
		originalEtcdQueryInterval = main.EtcdQueryInterval

		fakeStoreAdapter = fakestoreadapter.New()

		main.EtcdQueryInterval = 1 * time.Millisecond
	})

	AfterEach(func() {
		main.EtcdQueryInterval = originalEtcdQueryInterval
	})

	It("gets loggregator addresses (with port) from the store", func() {
		stopChan := make(chan struct{})
		defer close(stopChan)

		node := storeadapter.StoreNode{
			Key:   "healthstatus/loggregator/z1/loggregator/0",
			Value: []byte("1.2.3.4"),
		}
		fakeStoreAdapter.Create(node)

		provider := main.MakeProvider(fakeStoreAdapter, "healthstatus/loggregator", 3456, loggertesthelper.Logger())

		Eventually(provider.ServerAddresses).Should(ConsistOf("1.2.3.4:3456"))
	})
})
