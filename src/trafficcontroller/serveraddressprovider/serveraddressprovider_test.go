package serveraddressprovider_test

import (
	"time"
	"trafficcontroller/serveraddressprovider"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServerAddressProvider", func() {
	It("adds the configured port to the list's addresses", func() {
		list := fakeServerAddressList{addresses: []string{"1.2.3.4", "1.2.3.5"}}

		provider := serveraddressprovider.NewDynamicServerAddressProvider(list, uint32(7))

		Expect(provider.ServerAddresses()).To(Equal([]string{"1.2.3.4:7", "1.2.3.5:7"}))
	})
})

type fakeServerAddressList struct {
	addresses []string
}

func (fake fakeServerAddressList) Run(time.Duration) {}
func (fake fakeServerAddressList) Stop()             {}

func (fake fakeServerAddressList) GetAddresses() []string {
	return fake.addresses
}

func (fake fakeServerAddressList) DiscoverAddresses() {

}
