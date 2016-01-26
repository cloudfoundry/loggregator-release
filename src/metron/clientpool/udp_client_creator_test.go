package clientpool_test

import (
	"metron/clientpool"

	"github.com/cloudfoundry/gosteno"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("UDPClientCreator", func() {

	var (
		udpClientCreator *clientpool.UDPClientCreator
		logger           *gosteno.Logger
		address          string
	)

	BeforeEach(func() {
		logger = gosteno.NewLogger("TestLogger")
		address = "127.0.0.1:1234"
		udpClientCreator = clientpool.NewUDPClientCreator(logger)
	})

	Describe("CreateClient", func() {
		var (
			client    clientpool.Client
			createErr error
		)

		JustBeforeEach(func() {
			client, createErr = udpClientCreator.CreateClient(address)
		})
		It("makes clients", func() {
			Expect(createErr).ToNot(HaveOccurred())
			Expect(client.Address()).To(Equal(address))
			Expect(client.Scheme()).To(Equal("udp"))
		})

		Context("with an invalid address", func() {
			BeforeEach(func() {
				address = "I am definitely not an address"
			})

			It("returns an error and a nil client", func() {
				Expect(createErr).To(HaveOccurred())
				Expect(client).To(BeNil())
			})
		})

		Context("with a returned client", func() {
			var client clientpool.Client

			BeforeEach(func() {
				var err error
				client, err = udpClientCreator.CreateClient(address)
				Expect(err).ToNot(HaveOccurred())
			})

			It("can write", func() {
				message := []byte("foo")
				written, err := client.Write(message)
				Expect(err).ToNot(HaveOccurred())
				Expect(written).To(BeEquivalentTo(len(message)))
			})
		})
	})
})
