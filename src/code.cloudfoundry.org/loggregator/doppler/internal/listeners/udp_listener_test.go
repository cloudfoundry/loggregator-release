package listeners_test

import (
	"net"
	"strconv"

	"code.cloudfoundry.org/loggregator/doppler/internal/listeners"

	"github.com/apoydence/eachers/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/ginkgo/config"
)

var _ = Describe("AgentListener", func() {
	var (
		listener        *listeners.UDPListener
		dataChannel     <-chan []byte
		listenerStopped chan struct{}
		address         string
		mockBatcher     *mockBatcher
		mockChainer     *mockBatchCounterChainer
	)

	BeforeEach(func() {
		mockBatcher = newMockBatcher()
		mockChainer = newMockBatchCounterChainer()
		testhelpers.AlwaysReturn(mockBatcher.BatchCounterOutput, mockChainer)
		testhelpers.AlwaysReturn(mockChainer.SetTagOutput, mockChainer)

		listenerStopped = make(chan struct{})

		port := 3456 + config.GinkgoConfig.ParallelNode
		address = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
		listener, dataChannel = listeners.NewUDPListener(
			address,
			mockBatcher,
			"udpListener",
		)
		go func() {
			listener.Start()
			close(listenerStopped)
		}()
	})

	AfterEach(func() {
		listener.Stop()
		Eventually(listenerStopped).Should(BeClosed())
	})

	Context("with a listner running", func() {
		It("listens to the socket and forwards log lines", func() {
			expectedData := "Some Data"

			connection, err := net.Dial("udp", address)
			Expect(err).ToNot(HaveOccurred())

			f := func() int {
				connection.Write([]byte(expectedData))
				return len(dataChannel)
			}
			Eventually(f).ShouldNot(BeZero())

			var received []byte
			Expect(dataChannel).Should(Receive(&received))
			Expect(string(received)).To(Equal(expectedData))
		})
	})
})
