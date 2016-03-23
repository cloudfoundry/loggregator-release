package clientpool_test

import (
	"errors"
	"metron/clientpool"

	steno "github.com/cloudfoundry/gosteno"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DopplerPool", func() {
	var (
		pool   *clientpool.DopplerPool
		logger *steno.Logger

		mockClientCreator *mockClientCreator
		addresses         []string
	)

	BeforeEach(func() {
		logger = steno.NewLogger("TestLogger")

		addresses = []string{
			"pahost:paport",
			"pbhost:pbport",
		}

		mockClientCreator = newMockClientCreator()
	})

	JustBeforeEach(func() {
		pool = clientpool.NewDopplerPool(logger, mockClientCreator)
	})

	Describe("SetAddresses", func() {

		Context("with successful client creation", func() {

			var clients []clientpool.Client

			BeforeEach(func() {
				clients = []clientpool.Client{
					newMockClient(),
					newMockClient(),
				}
				close(mockClientCreator.CreateClientOutput.err)
				mockClientCreator.CreateClientOutput.client <- clients[0]
				mockClientCreator.CreateClientOutput.client <- clients[1]
			})

			It("creates a client for each address", func() {
				Expect(pool.Clients()).To(HaveLen(0))

				clients := pool.SetAddresses(addresses)
				Expect(clients).To(Equal(2))

				Eventually(mockClientCreator.CreateClientInput.url).Should(Receive(Equal(addresses[0])))
				Eventually(mockClientCreator.CreateClientInput.url).Should(Receive(Equal(addresses[1])))
				Expect(pool.Clients()).To(HaveLen(2))
			})

		})

		Context("with failed client creation", func() {
			var err error

			BeforeEach(func() {
				err = errors.New("failed client creation")

				mockClientCreator.CreateClientOutput.client <- nil
				mockClientCreator.CreateClientOutput.err <- err

				mockClientCreator.CreateClientOutput.client <- newMockClient()
				mockClientCreator.CreateClientOutput.err <- nil
			})

			It("doesn't include the failed client", func() {
				clients := pool.SetAddresses(addresses)
				Expect(clients).To(Equal(1))

				Expect(pool.Clients()).To(HaveLen(1))
			})
		})

		Context("with new addresses to fill pool", func() {
			var (
				clients []clientpool.Client
				client1 *mockClient
				client2 *mockClient
			)

			BeforeEach(func() {
				client1 = newMockClient()
				client2 = newMockClient()
				clients = []clientpool.Client{client1, client2}
				close(mockClientCreator.CreateClientOutput.err)
				mockClientCreator.CreateClientOutput.client <- clients[0]
				mockClientCreator.CreateClientOutput.client <- clients[1]
				client1.AddressOutput.ret0 <- "pahost:paport"
				client2.AddressOutput.ret0 <- "pbhost:pbport"
				close(client1.CloseOutput.ret0)
				close(client2.CloseOutput.ret0)
			})

			It("closes previous client connections", func() {
				Expect(pool.Clients()).To(HaveLen(0))

				pool.SetAddresses(addresses)
				Expect(pool.Clients()).To(HaveLen(2))

				newClient := newMockClient()
				mockClientCreator.CreateClientOutput.client <- newClient
				pool.SetAddresses([]string{"pchost:pcport"})
				Expect(pool.Clients()).To(HaveLen(1))
				Eventually(client1.CloseCalled).Should(Receive(BeTrue()))
				Eventually(client2.CloseCalled).Should(Receive(BeTrue()))
			})
		})
	})

	Describe("RandomClient", func() {

		Context("with non-empty client pool", func() {

			BeforeEach(func() {
				close(mockClientCreator.CreateClientOutput.err)
				mockClientCreator.CreateClientOutput.client <- newMockClient()
				mockClientCreator.CreateClientOutput.client <- newMockClient()
			})

			JustBeforeEach(func() {
				pool.SetAddresses(addresses)
			})

			It("returns a random client", func() {
				Expect(pool.RandomClient()).ToNot(BeNil())
			})

		})

		Context("with empty client pool", func() {
			JustBeforeEach(func() {
				pool.SetAddresses([]string{})
			})

			It("returns ErrorEmptyClientPool", func() {
				client, err := pool.RandomClient()

				Expect(client).To(BeNil())
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(clientpool.ErrorEmptyClientPool))
			})
		})
	})

	Describe("Size", func() {
		BeforeEach(func() {
			close(mockClientCreator.CreateClientOutput.err)
			mockClientCreator.CreateClientOutput.client <- newMockClient()
			mockClientCreator.CreateClientOutput.client <- newMockClient()
		})

		Context("with non-empty client pool", func() {

			JustBeforeEach(func() {
				pool.SetAddresses(addresses)
			})

			It("if non-empty returns the correct size", func() {
				Expect(pool.Size()).To(Equal(len(addresses)))
			})

		})

		Context("with empty client pool", func() {
			JustBeforeEach(func() {
				pool.SetAddresses([]string{})
			})

			It("if empty returns zero", func() {
				Expect(pool.Size()).To(Equal(0))
			})
		})
	})
})
