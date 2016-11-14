package dopplerforwarder_test

import (
	"errors"
	"metron/writers/dopplerforwarder"

	. "github.com/apoydence/eachers"
	"github.com/apoydence/eachers/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/dropsonde/signature"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

var _ = Describe("UDPWrapper", func() {
	var (
		envelope     *events.Envelope
		udpWrapper   *dopplerforwarder.UDPWrapper
		message      []byte
		sharedSecret []byte

		mockBatcher *mockMetricBatcher
		mockConn    *mockUDPConn
	)

	BeforeEach(func() {
		sharedSecret = []byte("secret")
		mockBatcher = newMockMetricBatcher()
		metrics.Initialize(nil, mockBatcher)

		mockConn = newMockUDPConn()

		envelope = &events.Envelope{
			Origin:     proto.String("fake-origin-1"),
			EventType:  events.Envelope_LogMessage.Enum(),
			LogMessage: factories.NewLogMessage(events.LogMessage_OUT, "message", "appid", "sourceType"),
		}
		udpWrapper = dopplerforwarder.NewUDPWrapper(mockConn, sharedSecret)

		var err error
		message, err = proto.Marshal(envelope)
		Expect(err).NotTo(HaveOccurred())
	})

	It("counts the number of bytes sent", func() {
		// Make sure the counter counts the sent byte count,
		// instead of len(message)
		sentLength := len(message)
		mockConn.WriteOutput.Ret0 <- nil

		err := udpWrapper.Write(message)
		Expect(err).NotTo(HaveOccurred())
		Eventually(mockBatcher.BatchAddCounterInput).Should(BeCalled(
			With("udp.sentByteCount", uint64(sentLength)),
		))
	})

	It("counts the number of messages sent", func() {
		mockConn.WriteOutput.Ret0 <- nil
		mockChainer := newMockBatchCounterChainer()
		testhelpers.AlwaysReturn(mockChainer.SetTagOutput, mockChainer)

		err := udpWrapper.Write(message, mockChainer)
		Expect(err).NotTo(HaveOccurred())
		Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
			With("udp.sentMessageCount"),
		))
		Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
			With("DopplerForwarder.sentMessages"),
		))
		Eventually(mockChainer.SetTagInput).Should(BeCalled(
			With("protocol", "udp"),
		))
		Eventually(mockChainer.IncrementCalled).Should(BeCalled())
	})

	It("increments transmitErrorCount *only* if client write fails", func() {
		mockConn.WriteOutput.Ret0 <- nil

		err := udpWrapper.Write(message)
		Expect(err).NotTo(HaveOccurred())
		Consistently(mockBatcher.BatchIncrementCounterInput).ShouldNot(BeCalled(
			With("udp.sendErrorCount"),
		))

		err = errors.New("Client Write Failed")
		mockConn.WriteOutput.Ret0 <- err

		err = udpWrapper.Write(message)
		Expect(err).To(HaveOccurred())
		Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
			With("udp.sendErrorCount"),
		))
	})

	It("signs and writes a message", func() {
		signedMessage := signature.SignMessage(message, sharedSecret)

		mockConn.WriteOutput.Ret0 <- nil

		udpWrapper.Write(message)

		Eventually(mockConn.WriteCalled).Should(HaveLen(1))
		Eventually(mockConn.WriteInput.Data).Should(Receive(Equal(signedMessage)))
	})

	Context("when client write fail", func() {
		BeforeEach(func() {
			mockConn.WriteOutput.Ret0 <- errors.New("failed")
		})

		It("returns an error", func() {
			err := udpWrapper.Write(message)
			Expect(err).To(HaveOccurred())
		})

		It("does not increment message count or sentMessages", func() {
			udpWrapper.Write(message)

			var name string
			Eventually(mockBatcher.BatchIncrementCounterInput.Name).Should(Receive(&name))
			Expect(name).To(Equal("udp.sendErrorCount"))
			Consistently(mockBatcher.BatchIncrementCounterInput).ShouldNot(BeCalled())

			Consistently(mockBatcher.BatchIncrementCounterInput).ShouldNot(BeCalled())
		})
	})
})
