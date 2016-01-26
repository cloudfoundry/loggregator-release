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

		client          *clientpool.TLSClient
		tlsClientConfig *tls.Config
	)

	BeforeEach(func() {
		logger = gosteno.NewLogger("test")

		var err error
		tlsServerConfig, err := listeners.NewTLSConfig("fixtures/server.crt", "fixtures/server.key", "fixtures/loggregator-ca.crt")
		Expect(err).NotTo(HaveOccurred())

		tlsListener, err = tls.Listen("tcp", "127.0.0.1:0", tlsServerConfig)
		Expect(err).NotTo(HaveOccurred())

		tlsClientConfig, err = listeners.NewTLSConfig("fixtures/client.crt", "fixtures/client.key", "fixtures/loggregator-ca.crt")
		Expect(err).NotTo(HaveOccurred())
		tlsClientConfig.ServerName = "doppler"

		client = clientpool.NewTLSClient(logger, tlsListener.Addr().String(), tlsClientConfig)
		Expect(client).ToNot(BeNil())
	})

	AfterEach(func() {
		tlsListener.Close()
		if client != nil {
			client.Close()
		}
	})

	Describe("Connect", func() {
		var connErr error

		JustBeforeEach(func() {
			connChan = acceptConnections(tlsListener)
			connErr = client.Connect()
		})

		Context("with a valid TLSListener", func() {
			It("returns a connection without error", func() {
				Expect(connErr).NotTo(HaveOccurred())
				Eventually(connChan).Should(Receive())
			})
		})

		Context("without a TLSListener", func() {
			BeforeEach(func() {
				Expect(tlsListener.Close()).ToNot(HaveOccurred())
			})

			It("returns an error", func() {
				Expect(connErr).To(HaveOccurred())
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

	Describe("Write", func() {
		var conn net.Conn

		BeforeEach(func() {
			connChan = acceptConnections(tlsListener)
			Expect(client.Connect()).ToNot(HaveOccurred())
			conn = <-connChan
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

func acceptConnections(listener net.Listener) chan net.Conn {
	connChan := make(chan net.Conn, 1)
	go func() {
		defer GinkgoRecover()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			err = conn.(*tls.Conn).Handshake()
			if err == nil {
				connChan <- conn
			}
		}
	}()
	return connChan
}
