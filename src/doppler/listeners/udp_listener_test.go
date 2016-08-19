package listeners_test

import (
	"doppler/listeners"
	"net"
	"strconv"

	. "github.com/apoydence/eachers"
	"github.com/apoydence/eachers/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
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
		loggertesthelper.TestLoggerSink.Clear()

		port := 3456 + config.GinkgoConfig.ParallelNode
		address = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
		listener, dataChannel = listeners.NewUDPListener(
			address,
			mockBatcher,
			loggertesthelper.Logger(),
			"udpListener",
		)
		go func() {
			listener.Start()
			close(listenerStopped)
		}()

		Eventually(loggertesthelper.TestLoggerSink.LogContents).Should(ContainSubstring("UDP listener listening on port " + address))
	})

	AfterEach(func() {
		listener.Stop()
		Eventually(listenerStopped).Should(BeClosed())
	})

	Context("with a listner running", func() {
		It("listens to the socket and forwards log lines", func() {
			expectedData := "Some Data"
			otherData := "More stuff"

			connection, err := net.Dial("udp", address)

			_, err = connection.Write([]byte(expectedData))
			Expect(err).To(BeNil())

			var received []byte
			Eventually(dataChannel).Should(Receive(&received))
			Expect(string(received)).To(Equal(expectedData))

			_, err = connection.Write([]byte(otherData))
			Expect(err).To(BeNil())

			Eventually(dataChannel).Should(Receive(&received))
			Expect(string(received)).To(Equal(otherData))
		})
	})

	Context("dropsonde metric emission", func() {
		It("issues intended metrics", func() {
			expectedData := "Some Data"

			connection, err := net.Dial("udp", address)
			_, err = connection.Write([]byte(expectedData))
			Expect(err).NotTo(HaveOccurred())
			Eventually(dataChannel).Should(Receive())

			Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
				With("dropsondeListener.receivedMessageCount"),
				With("udpListener.receivedMessageCount"),
				With("listeners.totalReceivedMessageCount"),
			))
			Eventually(mockBatcher.BatchAddCounterInput).Should(BeCalled(
				With("dropsondeListener.receivedByteCount", uint64(len(expectedData))),
				With("udpListener.receivedByteCount", uint64(len(expectedData))),
				With("listeners.totalReceivedByteCount", uint64(len(expectedData))),
			))
		})
	})
})
