package clientreader_test

import (
	"doppler/dopplerservice"
	"metron/clientreader"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("clientreader", func() {
	var (
		protocols  []string
		poolMocks  map[string]*mockClientPool
		event      dopplerservice.Event
		clientPool map[string]clientreader.ClientPool
	)

	BeforeEach(func() {
		protocols = nil
		poolMocks = make(map[string]*mockClientPool)
		poolMocks["udp"] = newMockClientPool()
		poolMocks["tcp"] = newMockClientPool()
		poolMocks["tls"] = newMockClientPool()
		event = dopplerservice.Event{}
		clientPool = make(map[string]clientreader.ClientPool)
	})

	JustBeforeEach(func() {
		for _, proto := range protocols {
			clientPool[proto] = poolMocks[proto]
		}
	})

	Describe("Read", func() {
		Context("with TLS as the only protocol", func() {
			BeforeEach(func() {
				protocols = []string{"tls"}
			})

			It("doesn't panic if there are tls dopplers", func() {
				event = dopplerservice.Event{
					UDPDopplers: []string{},
					TLSDopplers: []string{"10.0.0.1"},
				}

				l := len(event.TLSDopplers)
				poolMocks["tls"].SetAddressesOutput.ret0 <- l
				Expect(clientreader.Read(clientPool, protocols, event)).To(Equal("tls"))
				Eventually(poolMocks["tls"].SetAddressesCalled).Should(Receive())
			})

			It("panics if there are no tls dopplers", func() {
				event = dopplerservice.Event{
					UDPDopplers: []string{"1.1.1.1.", "2.2.2.2"},
					TLSDopplers: []string{},
				}
				l := len(event.TLSDopplers)
				poolMocks["tls"].SetAddressesOutput.ret0 <- l

				Expect(func() {
					clientreader.Read(clientPool, protocols, event)
				}).To(Panic())
			})

			It("panics if none of the dopplers can be connected to", func() {
				event = dopplerservice.Event{
					UDPDopplers: []string{},
					TLSDopplers: []string{"10.0.0.1"},
				}

				poolMocks["tls"].SetAddressesOutput.ret0 <- 0
				Expect(func() {
					clientreader.Read(clientPool, protocols, event)
				}).To(Panic())
			})
		})

		Context("with UDP as the only protocol", func() {
			BeforeEach(func() {
				protocols = []string{"udp"}
			})

			It("doesn't panic for udp only dopplers", func() {
				event := dopplerservice.Event{
					UDPDopplers: []string{"10.0.0.1"},
					TLSDopplers: []string{},
				}
				l := len(event.UDPDopplers)
				poolMocks["udp"].SetAddressesOutput.ret0 <- l

				Expect(clientreader.Read(clientPool, protocols, event)).To(Equal("udp"))
				Eventually(poolMocks["udp"].SetAddressesCalled).Should(Receive())
			})
		})

		Context("with TCP as the only protocol", func() {
			BeforeEach(func() {
				protocols = []string{"tcp"}
			})

			It("doesn't panic if there are tcp dopplers", func() {
				event = dopplerservice.Event{
					UDPDopplers: []string{},
					TLSDopplers: []string{"10.0.0.1"},
					TCPDopplers: []string{"10.0.0.1"},
				}

				l := len(event.TCPDopplers)
				poolMocks["tcp"].SetAddressesOutput.ret0 <- l
				Expect(clientreader.Read(clientPool, protocols, event)).To(Equal("tcp"))
				Eventually(poolMocks["tcp"].SetAddressesCalled).Should(Receive())
			})

			It("panics if there are no tcp dopplers", func() {
				event = dopplerservice.Event{
					UDPDopplers: []string{},
					TLSDopplers: []string{"10.0.0.1"},
				}

				l := len(event.TCPDopplers)
				poolMocks["tcp"].SetAddressesOutput.ret0 <- l
				Expect(func() {
					clientreader.Read(clientPool, protocols, event)
				}).To(Panic())
			})
		})

		Context("with multiple protocols", func() {
			BeforeEach(func() {
				protocols = []string{"tls", "tcp", "udp"}
				event = dopplerservice.Event{
					UDPDopplers: []string{"10.0.0.1"},
					TLSDopplers: []string{"10.0.0.2"},
					TCPDopplers: []string{"10.0.0.3"},
				}

				poolMocks["udp"].SetAddressesOutput.ret0 <- 1
				poolMocks["tls"].SetAddressesOutput.ret0 <- 1
				poolMocks["tcp"].SetAddressesOutput.ret0 <- 1
			})

			It("calls SetAddresses on the first protocol only", func() {
				Expect(clientreader.Read(clientPool, protocols, event)).To(Equal("tls"))
				Eventually(poolMocks["tls"].SetAddressesCalled).Should(Receive())
				Eventually(poolMocks["tls"].SetAddressesInput.addresses).Should(Receive(Equal([]string{
					"10.0.0.2",
				})))
				Consistently(poolMocks["udp"].SetAddressesCalled).ShouldNot(Receive())
				Consistently(poolMocks["tcp"].SetAddressesCalled).ShouldNot(Receive())
			})

			Context("with etcd containing only udp dopplers", func() {
				BeforeEach(func() {
					event = dopplerservice.Event{
						UDPDopplers: []string{"10.0.0.1"},
					}
				})

				It("skips tls and tcp", func() {
					Expect(clientreader.Read(clientPool, protocols, event)).To(Equal("udp"))
					Eventually(poolMocks["udp"].SetAddressesCalled).Should(Receive())
					Eventually(poolMocks["udp"].SetAddressesInput.addresses).Should(Receive(Equal([]string{
						"10.0.0.1",
					})))
					Consistently(poolMocks["tls"].SetAddressesCalled).ShouldNot(Receive())
					Consistently(poolMocks["tcp"].SetAddressesCalled).ShouldNot(Receive())
				})
			})
		})
	})
})
