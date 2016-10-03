package clientpool_test

import (
	"crypto/tls"
	"doppler/listeners"
	"metron/clientpool"
	"net"
	"time"

	"github.com/cloudfoundry/gosteno"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TCPClient", func() {
	var (
		logger      *gosteno.Logger
		listener    net.Listener
		connections chan net.Conn

		client *clientpool.TCPClient
	)

	BeforeEach(func() {
		logger = gosteno.NewLogger("test")
	})

	AfterEach(func() {
		if listener != nil {
			listener.Close()
		}
		if client != nil {
			client.Close()
		}
	})

	Context("with a client and server configured without TLS", func() {
		BeforeEach(func() {
			var err error
			listener, err = net.Listen("tcp", "127.0.0.1:0")
			Expect(err).NotTo(HaveOccurred())

			client = clientpool.NewTCPClient(logger, listener.Addr().String(), nil)
			Expect(client).ToNot(BeNil())
		})

		Describe("Connect", func() {
			var connErr error

			JustBeforeEach(func() {
				connections = acceptConnections(listener)
				connErr = client.Connect()
			})

			Context("with a valid listener", func() {
				It("returns a connection without error", func() {
					Expect(connErr).NotTo(HaveOccurred())
					Eventually(connections).Should(Receive())
				})
			})

			Context("without a listener", func() {
				BeforeEach(func() {
					Expect(listener.Close()).ToNot(HaveOccurred())
				})

				It("returns an error", func() {
					Expect(connErr).To(HaveOccurred())
					Consistently(connections).ShouldNot(Receive())
				})
			})
		})

		Describe("Scheme", func() {
			It("returns tcp", func() {
				Expect(client.Scheme()).To(Equal("tcp"))
			})
		})

		Describe("Address", func() {
			It("returns the address", func() {
				Expect(client.Address()).To(Equal(listener.Addr().String()))
			})
		})

		Describe("Write", func() {
			var conn net.Conn

			BeforeEach(func() {
				connections = acceptConnections(listener)
				Expect(client.Connect()).ToNot(HaveOccurred())
				conn = <-connections
			})

			It("sends data", func() {
				_, err := client.Write([]byte("abc"))
				Expect(err).NotTo(HaveOccurred())
				bytes := make([]byte, 10)
				n, err := conn.Read(bytes)
				Expect(err).NotTo(HaveOccurred())
				Expect(bytes[:n]).To(Equal([]byte("abc")))
			})

			Context("when write is called with an empty byte slice", func() {
				var writeErr error

				JustBeforeEach(func() {
					_, writeErr = client.Write([]byte{})
				})

				It("does not send", func() {
					Expect(writeErr).NotTo(HaveOccurred())

					bytes := make([]byte, 10)
					conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
					_, err := conn.Read(bytes)
					Expect(err).To(HaveOccurred())
					opErr := err.(*net.OpError)
					Expect(opErr.Timeout()).To(BeTrue())
				})
			})

			Context("when the connection is closed", func() {
				BeforeEach(func() {
					Expect(client.Close()).ToNot(HaveOccurred())
				})

				It("reconnects and sends", func() {
					_, err := client.Write([]byte("abc"))
					Expect(err).NotTo(HaveOccurred())

					Eventually(connections).Should(Receive(&conn))
					Expect(conn).NotTo(BeNil())
					bytes := make([]byte, 10)
					n, err := conn.Read(bytes)
					Expect(err).NotTo(HaveOccurred())
					Expect(bytes[:n]).To(Equal([]byte("abc")))
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

	Context("with a client and server configured with TLS", func() {
		var (
			tlsClientConfig *tls.Config
		)

		BeforeEach(func() {
			var err error
			tlsServerConfig, err := listeners.NewTLSConfig("fixtures/server.crt", "fixtures/server.key", "fixtures/loggregator-ca.crt")
			Expect(err).NotTo(HaveOccurred())

			listener, err = tls.Listen("tcp", "127.0.0.1:0", tlsServerConfig)
			Expect(err).NotTo(HaveOccurred())

			tlsClientConfig, err = listeners.NewTLSConfig("fixtures/client.crt", "fixtures/client.key", "fixtures/loggregator-ca.crt")
			Expect(err).NotTo(HaveOccurred())
			tlsClientConfig.ServerName = "doppler"

			client = clientpool.NewTCPClient(logger, listener.Addr().String(), tlsClientConfig)
			Expect(client).ToNot(BeNil())
		})

		Describe("Connect", func() {
			var connErr error

			JustBeforeEach(func() {
				connections = acceptConnections(listener)
				connErr = client.Connect()
			})

			Context("with a valid listener", func() {
				It("returns a connection without error", func() {
					Expect(connErr).NotTo(HaveOccurred())
					Eventually(connections, 5).Should(Receive())
				})
			})

			Context("without a listener", func() {
				BeforeEach(func() {
					Expect(listener.Close()).ToNot(HaveOccurred())
				})

				It("returns an error", func() {
					Expect(connErr).To(HaveOccurred())
					Consistently(connections).ShouldNot(Receive())
				})
			})
		})

		Describe("Scheme", func() {
			It("returns tls", func() {
				Expect(client.Scheme()).To(Equal("tls"))
			})
		})
	})
})

func acceptConnections(listener net.Listener) chan net.Conn {
	connChan := make(chan net.Conn, 1)
	go func() {
		defer GinkgoRecover()
		for {
			conn, err := listener.Accept()

			if err != nil {
				return
			}

			switch c := conn.(type) {
			case *tls.Conn:
				err = c.Handshake()
			}

			if err == nil {
				connChan <- conn
			}
		}
	}()
	return connChan
}
