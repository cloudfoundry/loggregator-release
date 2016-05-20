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
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

var _ = Describe("UDPWrapper", func() {
	var (
		client       *mockClient
		envelope     *events.Envelope
		udpWrapper   *dopplerforwarder.UDPWrapper
		logger       *gosteno.Logger
		message      []byte
		sharedSecret []byte
		mockBatcher  *mockMetricBatcher
	)

	BeforeEach(func() {
		sharedSecret = []byte("secret")
		mockBatcher = newMockMetricBatcher()
		metrics.Initialize(nil, mockBatcher)

		client = newMockClient()
		envelope = &events.Envelope{
			Origin:     proto.String("fake-origin-1"),
			EventType:  events.Envelope_LogMessage.Enum(),
			LogMessage: factories.NewLogMessage(events.LogMessage_OUT, "message", "appid", "sourceType"),
		}
		logger = loggertesthelper.Logger()
		udpWrapper = dopplerforwarder.NewUDPWrapper(sharedSecret, logger)

		var err error
		message, err = proto.Marshal(envelope)
		Expect(err).NotTo(HaveOccurred())

	})

	It("counts the number of bytes sent", func() {
		// Make sure the counter counts the sent byte count,
		// instead of len(message)
		sentLength := len(message) - 3
		client.WriteOutput.SentLength <- sentLength
		client.WriteOutput.Err <- nil

		err := udpWrapper.Write(client, message)
		Expect(err).NotTo(HaveOccurred())
		Eventually(mockBatcher.BatchAddCounterInput).Should(BeCalled(
			With("udp.sentByteCount", uint64(sentLength)),
		))
	})

	It("counts the number of messages sent", func() {
		client.WriteOutput.SentLength <- len(message)
		client.WriteOutput.Err <- nil
		mockChainer := newMockBatchCounterChainer()
		testhelpers.AlwaysReturn(mockChainer.SetTagOutput, mockChainer)

		err := udpWrapper.Write(client, message, mockChainer)
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
		client.WriteOutput.SentLength <- len(message)
		client.WriteOutput.Err <- nil

		err := udpWrapper.Write(client, message)
		Expect(err).NotTo(HaveOccurred())
		Consistently(mockBatcher.BatchIncrementCounterInput).ShouldNot(BeCalled(
			With("udp.sendErrorCount"),
		))

		err = errors.New("Client Write Failed")
		client.WriteOutput.SentLength <- 0
		client.WriteOutput.Err <- err

		err = udpWrapper.Write(client, message)
		Expect(err).To(HaveOccurred())
		Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
			With("udp.sendErrorCount"),
		))
	})

	It("signs and writes a message", func() {
		signedMessage := signature.SignMessage(message, sharedSecret)

		client.WriteOutput.SentLength <- len(signedMessage)
		client.WriteOutput.Err <- nil

		udpWrapper.Write(client, message)

		Eventually(client.WriteCalled).Should(HaveLen(1))
		Eventually(client.WriteInput.Message).Should(Receive(Equal(signedMessage)))
		Consistently(client.CloseCalled).ShouldNot(Receive())
	})

	Context("when client write fail", func() {
		BeforeEach(func() {
			client.WriteOutput.SentLength <- 0
			client.WriteOutput.Err <- errors.New("failed")
		})

		It("returns an error", func() {
			err := udpWrapper.Write(client, message)
			Expect(err).To(HaveOccurred())
		})

		It("does not increment message count or sentMessages", func() {
			udpWrapper.Write(client, message)

			var name string
			Eventually(mockBatcher.BatchIncrementCounterInput.Name).Should(Receive(&name))
			Expect(name).To(Equal("udp.sendErrorCount"))
			Consistently(mockBatcher.BatchIncrementCounterInput).ShouldNot(BeCalled())

			Consistently(mockBatcher.BatchIncrementCounterInput).ShouldNot(BeCalled())
		})

		It("never calls close", func() {
			udpWrapper.Write(client, message)
			Consistently(client.CloseCalled).ShouldNot(Receive())
		})
	})
})
