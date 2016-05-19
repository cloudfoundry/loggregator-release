package dopplerforwarder_test

import (
	"errors"
	"metron/writers/dopplerforwarder"

	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/dropsonde/signature"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

var _ = Describe("UDPWrapper", func() {
	var (
		sender       *fake.FakeMetricSender
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
		sender = fake.NewFakeMetricSender()
		mockBatcher = newMockMetricBatcher()
		metrics.Initialize(sender, mockBatcher)
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
		client.WriteOutput.sentLength <- sentLength
		client.WriteOutput.err <- nil

		err := udpWrapper.Write(client, message)
		Expect(err).NotTo(HaveOccurred())
		Eventually(mockBatcher.BatchAddCounterInput).Should(BeCalled(
			With("udp.sentByteCount", uint64(sentLength)),
		))
	})

	It("counts the number of messages sent", func() {
		client.WriteOutput.sentLength <- len(message)
		client.WriteOutput.err <- nil

		err := udpWrapper.Write(client, message)
		Expect(err).NotTo(HaveOccurred())
		Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
			With("udp.sentMessageCount"),
		))
		Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
			With("DopplerForwarder.sentMessages"),
		))
	})

	It("increments transmitErrorCount *only* if client write fails", func() {
		client.WriteOutput.sentLength <- len(message)
		client.WriteOutput.err <- nil

		err := udpWrapper.Write(client, message)
		Expect(err).NotTo(HaveOccurred())
		Consistently(func() uint64 {
			return sender.GetCounter("udp.sendErrorCount")
		}).Should(BeEquivalentTo(0))

		err = errors.New("Client Write Failed")
		client.WriteOutput.sentLength <- 0
		client.WriteOutput.err <- err

		err = udpWrapper.Write(client, message)
		Expect(err).To(HaveOccurred())
		Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
			With("udp.sendErrorCount"),
		))
	})

	It("signs and writes a message", func() {
		signedMessage := signature.SignMessage(message, sharedSecret)

		client.WriteOutput.sentLength <- len(signedMessage)
		client.WriteOutput.err <- nil

		udpWrapper.Write(client, message)

		Eventually(client.WriteCalled).Should(HaveLen(1))
		Eventually(client.WriteInput.message).Should(Receive(Equal(signedMessage)))
		Consistently(client.CloseCalled).ShouldNot(Receive())
	})

	Context("when client write fail", func() {
		BeforeEach(func() {
			client.WriteOutput.sentLength <- 0
			client.WriteOutput.err <- errors.New("failed")
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
		})

		It("never calls close", func() {
			udpWrapper.Write(client, message)
			Consistently(client.CloseCalled).ShouldNot(Receive())
		})
	})
})
