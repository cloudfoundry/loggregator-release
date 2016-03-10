package clientpool_test

import (
	"metron/clientpool"
	"net"

	"github.com/cloudfoundry/gosteno"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("UDP Client", func() {
	var (
		client      *clientpool.UDPClient
		udpListener *net.UDPConn
	)

	BeforeEach(func() {
		udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
		Expect(err).NotTo(HaveOccurred())
		udpListener, err = net.ListenUDP("udp", udpAddr)
		Expect(err).NotTo(HaveOccurred())

		client, err = clientpool.NewUDPClient(gosteno.NewLogger("TestLogger"), udpListener.LocalAddr().String())
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		client.Close()
		udpListener.Close()
	})

	Describe("NewUDPClient", func() {
		Context("when the address is invalid", func() {
			It("returns an error", func() {
				_, err := clientpool.NewUDPClient(gosteno.NewLogger("TestLogger"), "127.0.0.1:abc")
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Scheme", func() {
		It("returns udp", func() {
			Expect(client.Scheme()).To(Equal("udp"))
		})
	})

	Describe("Address", func() {
		It("returns the address", func() {
			Expect(client.Address()).To(Equal(udpListener.LocalAddr().String()))
		})
	})

	Describe("Connect", func() {
		var connErr error

		JustBeforeEach(func() {
			connErr = client.Connect()
		})

		Context("with a valid UDP address", func() {
			It("returns a nil error", func() {
				Expect(connErr).ToNot(HaveOccurred())
			})
		})
	})

	Describe("Write", func() {

		Context("with a valid connection", func() {

			BeforeEach(func() {
				Expect(client.Connect()).ToNot(HaveOccurred())
			})

			It("sends log messages to loggregator", func() {
				expectedOutput := []byte("Important Testmessage")

				_, err := client.Write(expectedOutput)
				Expect(err).NotTo(HaveOccurred())

				buffer := make([]byte, 4096)
				readCount, _, err := udpListener.ReadFromUDP(buffer)
				Expect(err).NotTo(HaveOccurred())

				received := string(buffer[:readCount])
				Expect(received).To(Equal(string(expectedOutput)))
			})

			It("doesn't send empty data", func() {
				bufferSize := 4096
				firstMessage := []byte("")
				secondMessage := []byte("hi")

				_, err := client.Write(firstMessage)
				Expect(err).NotTo(HaveOccurred())
				_, err = client.Write(secondMessage)
				Expect(err).NotTo(HaveOccurred())

				buffer := make([]byte, bufferSize)
				readCount, _, err := udpListener.ReadFromUDP(buffer)
				Expect(err).NotTo(HaveOccurred())

				received := string(buffer[:readCount])
				Expect(received).To(Equal(string(secondMessage)))
			})
		})

		Context("with a nil connection", func() {
			It("returns an error and zero bytes written", func() {
				bytesWritten, err := client.Write([]byte("some message"))
				Expect(err).To(HaveOccurred())
				Expect(bytesWritten).To(BeEquivalentTo(0))
			})
		})
	})

	Describe("Close", func() {
		It("can be called multiple times", func() {
			done := make(chan struct{})
			go func() {
				client.Close()
				client.Close()
				close(done)
			}()
			Eventually(done).Should(BeClosed())
		})
	})
})
