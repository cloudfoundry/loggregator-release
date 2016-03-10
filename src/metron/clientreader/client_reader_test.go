package clientreader_test

import (
	"doppler/dopplerservice"
	"metron/clientreader"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	preferredProtocol string
	mockPool          *mockClientPool
	event             dopplerservice.Event
)

var _ = Describe("clientreader", func() {
	BeforeEach(func() {
		mockPool = newMockClientPool()
	})
	Describe("Read", func() {
		Context("TLS PreferredProtocol", func() {
			BeforeEach(func() {
				preferredProtocol = "tls"
			})

			It("doesn't panic if there are tls dopplers", func() {
				event = dopplerservice.Event{
					UDPDopplers: []string{},
					TLSDopplers: []string{"10.0.0.1"}}

				l := len(event.TLSDopplers)
				mockPool.SetAddressesOutput.ret0 <- l
				Expect(func() { clientreader.Read(mockPool, preferredProtocol, event) }).ToNot(Panic())
				Eventually(mockPool.SetAddressesCalled).Should(Receive())
			})

			It("panics if there no tls dopplers", func() {
				event = dopplerservice.Event{
					UDPDopplers: []string{"1.1.1.1.", "2.2.2.2"},
					TLSDopplers: []string{},
				}
				l := len(event.TLSDopplers)
				mockPool.SetAddressesOutput.ret0 <- l

				Expect(func() { clientreader.Read(mockPool, preferredProtocol, event) }).To(Panic())
				Eventually(mockPool.SetAddressesCalled).Should(Receive())
			})

		})
		Context("UDP PreferredProtocol", func() {
			BeforeEach(func() {
				preferredProtocol = "udp"
			})
			It("doesn't panic for udp only dopplers", func() {
				event := dopplerservice.Event{
					UDPDopplers: []string{"10.0.0.1"},
					TLSDopplers: []string{},
				}
				l := len(event.UDPDopplers)
				mockPool.SetAddressesOutput.ret0 <- l

				Expect(func() { clientreader.Read(mockPool, preferredProtocol, event) }).ToNot(Panic())
				Eventually(mockPool.SetAddressesCalled).Should(Receive())
			})

		})

	})
})
