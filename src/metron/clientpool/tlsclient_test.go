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

var _ = Describe("TLS Client", func() {
	var (
		logger      *gosteno.Logger
		tlsListener net.Listener
		connChan    chan net.Conn

		client clientpool.Client
	)

	BeforeEach(func() {
		logger = gosteno.NewLogger("test")

		var err error
		tlsServerConfig, err := listeners.NewTLSConfig("fixtures/server.crt", "fixtures/server.key", "fixtures/loggregator-ca.crt")
		Expect(err).NotTo(HaveOccurred())
		tlsListener, err = tls.Listen("tcp", "127.0.0.1:0", tlsServerConfig)
		Expect(err).NotTo(HaveOccurred())

		connChan = make(chan net.Conn, 1)

		tlsListener := tlsListener
		connChan := connChan
		go func() {
			defer GinkgoRecover()
			for {
				conn, err := tlsListener.Accept()
				if err != nil {
					return
				}

				err = conn.(*tls.Conn).Handshake()
				if err == nil {
					connChan <- conn
				}
			}
		}()
	})

	JustBeforeEach(func() {
		tlsClientConfig, err := listeners.NewTLSConfig("fixtures/client.crt", "fixtures/client.key", "fixtures/loggregator-ca.crt")
		Expect(err).NotTo(HaveOccurred())
		tlsClientConfig.ServerName = "doppler"

		client, err = clientpool.NewTLSClient(logger, tlsListener.Addr().String(), tlsClientConfig)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		tlsListener.Close()
		if client != nil {
			client.Close()
		}
	})

	Describe("NewTLSClient", func() {
		It("attempts to connect", func() {
			Expect(client).NotTo(BeNil())
			Eventually(connChan).Should(Receive())
		})

		Context("when the connect fails", func() {
			BeforeEach(func() {
				tlsListener.Close()
			})

			It("returns a client", func() {
				Expect(client).NotTo(BeNil())
				Consistently(connChan).ShouldNot(Receive())
			})
		})
	})

	Describe("Scheme", func() {
		It("returns tls", func() {
			Expect(client.Scheme()).To(Equal("tls"))
		})
	})

	Describe("Address", func() {
		It("returns the address", func() {
			Expect(client.Address()).To(Equal(tlsListener.Addr().String()))
		})
	})

	Describe("Writing data", func() {
		var conn net.Conn

		JustBeforeEach(func() {
			Eventually(connChan).Should(Receive(&conn))
			Expect(conn).NotTo(BeNil())
		})

		It("sends data", func() {
			_, err := client.Write([]byte("abc"))
			Expect(err).NotTo(HaveOccurred())
			bytes := make([]byte, 10)
			n, err := conn.Read(bytes)
			Expect(err).NotTo(HaveOccurred())
			Expect(bytes[:n]).To(Equal([]byte("abc")))
		})

		Context("when there is no data", func() {
			It("does not send", func() {
				_, err := client.Write([]byte(""))
				Expect(err).NotTo(HaveOccurred())

				bytes := make([]byte, 10)
				conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
				_, err = conn.Read(bytes)
				Expect(err).To(HaveOccurred())
				opErr := err.(*net.OpError)
				Expect(opErr.Timeout()).To(BeTrue())
			})
		})

		Context("when the connection is closed", func() {
			It("reconnects and sends", func() {
				client.Close()
				_, err := client.Write([]byte("abc"))
				Expect(err).NotTo(HaveOccurred())

				Eventually(connChan).Should(Receive(&conn))
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
