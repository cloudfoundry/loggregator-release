package serveraddressprovider_test

import (
	"trafficcontroller/serveraddressprovider"

	"sync"

	"github.com/cloudfoundry/storeadapter/fakestoreadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServerAddressProvider", func() {
	Describe("Start", func() {
		var fakeStoreAdapter *fakestoreadapter.FakeStoreAdapter

		BeforeEach(func() {
			fakeStoreAdapter = fakestoreadapter.New()
		})

		It("discovers loggregator addresses upfront, before starting the run loop", func() {
			addressList := &fakeServerAddressList{
				addressesToReturn: [][]string{
					[]string{"1.2.3.4"},
				},
			}
			provider := serveraddressprovider.NewDynamicServerAddressProvider(addressList, uint32(3456))

			provider.Start()

			Expect(provider.ServerAddresses()).To(ConsistOf("1.2.3.4:3456"))
		})

		It("periodically gets loggregator addresses (with port) from the store", func() {
			addressList := &fakeServerAddressList{
				addressesToReturn: [][]string{
					[]string{},
					[]string{"1.2.3.4"},
				},
			}
			provider := serveraddressprovider.NewDynamicServerAddressProvider(addressList, uint32(3456))

			provider.Start()

			Eventually(provider.ServerAddresses).Should(ConsistOf("1.2.3.4:3456"))
		})
	})
})

type fakeServerAddressList struct {
	addresses          []string
	addressesToReturn  [][]string
	numCalls           int
	runCalled          bool
	getAddressesCalled bool
	sync.Mutex
}

func (fake *fakeServerAddressList) Run() {
	fake.Lock()
	defer fake.Unlock()

	for _, addresses := range fake.addressesToReturn {
		fake.addresses = addresses
	}
}

func (fake *fakeServerAddressList) Stop() {}

func (fake *fakeServerAddressList) GetAddresses() []string {
	fake.Lock()
	defer fake.Unlock()
	return fake.addresses
}

func (fake *fakeServerAddressList) DiscoverAddresses() {
	if fake.addressesToReturn == nil {
		return
	}
	fake.addresses = fake.addressesToReturn[fake.numCalls]
	fake.numCalls++
}
