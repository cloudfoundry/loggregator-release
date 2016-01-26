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

				pool.SetAddresses(addresses)

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
				pool.SetAddresses(addresses)

				Expect(pool.Clients()).To(HaveLen(1))
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
})
