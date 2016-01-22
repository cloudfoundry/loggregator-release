package dopplerforwarder_test

import (
	"errors"
	"metron/writers/dopplerforwarder"
	"time"

	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/dropsonde/signature"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("UDPWrapper", func() {
	var (
		sender       *fake.FakeMetricSender
		client       *mockClient
		envelope     *events.Envelope
		udpWrapper   *dopplerforwarder.UDPWrapper
		message      []byte
		sharedSecret []byte
	)

	BeforeEach(func() {
		sharedSecret = []byte("secret")
		sender = fake.NewFakeMetricSender()
		metrics.Initialize(sender, metricbatcher.New(sender, time.Millisecond*10))
		client = newMockClient()
		envelope = &events.Envelope{
			Origin:     proto.String("fake-origin-1"),
			EventType:  events.Envelope_LogMessage.Enum(),
			LogMessage: factories.NewLogMessage(events.LogMessage_OUT, "message", "appid", "sourceType"),
		}
		udpWrapper = dopplerforwarder.NewUDPWrapper(sharedSecret)

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
		Eventually(func() uint64 {
			return sender.GetCounter("udp.sentByteCount")
		}).Should(BeEquivalentTo(sentLength))
	})

	It("counts the number of messages sent", func() {
		client.WriteOutput.sentLength <- len(message)
		client.WriteOutput.err <- nil

		err := udpWrapper.Write(client, message)
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() uint64 {
			return sender.GetCounter("udp.sentMessageCount")
		}).Should(BeEquivalentTo(1))
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
		Eventually(func() uint64 {
			return sender.GetCounter("udp.sendErrorCount")
		}).Should(BeEquivalentTo(1))
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

			Consistently(func() uint64 { return sender.GetCounter("udp.sentMessageCount") }).Should(BeZero())
			Consistently(func() uint64 { return sender.GetCounter("udp.sentByteCount") }).Should(BeZero())
			Eventually(func() uint64 { return sender.GetCounter("udp.sendErrorCount") }).Should(BeEquivalentTo(1))
		})

		It("never calls close", func() {
			udpWrapper.Write(client, message)
			Consistently(client.CloseCalled).ShouldNot(Receive())
		})
	})
})
