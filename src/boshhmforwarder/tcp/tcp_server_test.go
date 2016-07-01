package tcp_test

import (
	"boshhmforwarder/tcp"

	"fmt"
	"net"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tcp", func() {
	var (
		incoming chan string
		port     int
	)

	Context("Open TCP Port", func() {
		BeforeEach(func() {
			var err error
			incoming = make(chan string)
			port = 4000
			go func() {
				err = tcp.Open(port, incoming)
				Expect(err).ToNot(HaveOccurred())
			}()
			Eventually(func() error {
				conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))

				if err == nil && conn != nil {
					defer conn.Close()
				}

				return err
			}, 100).ShouldNot(HaveOccurred())
		})

		It("opens the TCP Port", func() {
			expectedMessage := "put LoggregatorDeaAgent.numCpus 1440704981 2 deployment=cf-dijon index=0 ip=10.10.16.31 job=LoggregatorDeaAgent name=LoggregatorDeaAgent/0 role=core status=3xx component=uaa"
			_, err := sendTCP(port, expectedMessage)
			Expect(err).ToNot(HaveOccurred())

			var message string
			Eventually(incoming, 60).Should(Receive(&message))
			Expect(message).To(Equal(expectedMessage))
		})
	})

	Context("when the port is busy", func() {

		BeforeEach(func() {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			Expect(err).ToNot(HaveOccurred())
			port = ln.Addr().(*net.TCPAddr).Port
			incoming = make(chan string)
		})

		It("reports an error", func(done Done) {
			defer close(done)
			err := tcp.Open(port, incoming)
			Expect(err).To(HaveOccurred())
		}, 1)
	})

})

func sendTCP(port int, message string) (int, error) {
	var conn net.Conn
	var err error
	Eventually(func() error {
		conn, err = net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		return err
	}, 5).ShouldNot(HaveOccurred())

	defer conn.Close()
	return fmt.Fprintf(conn, "%s\n", message)
}
