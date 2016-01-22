package dopplerforwarder_test

import (
	"metron/writers/dopplerforwarder"
	"time"

	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TLSWrapper", func() {
	var (
		sender     *fake.FakeMetricSender
		client     *mockClient
		envelope   *events.Envelope
		tlsWrapper *dopplerforwarder.TLSWrapper
		message    []byte
	)

	BeforeEach(func() {
		sender = fake.NewFakeMetricSender()
		metrics.Initialize(sender, metricbatcher.New(sender, time.Millisecond*10))
		client = newMockClient()
		envelope = &events.Envelope{
			Origin:     proto.String("fake-origin-1"),
			EventType:  events.Envelope_LogMessage.Enum(),
			LogMessage: factories.NewLogMessage(events.LogMessage_OUT, "message", "appid", "sourceType"),
		}
		tlsWrapper = dopplerforwarder.NewTLSWrapper()

		var err error
		message, err = proto.Marshal(envelope)
		Expect(err).NotTo(HaveOccurred())

	})

	It("counts the number of bytes sent", func() {

		sentLength := len(message) - 3
		client.WriteOutput.sentLength <- sentLength
		client.WriteOutput.err <- nil

		err := tlsWrapper.Write(client, message)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() uint64 {
			return sender.GetCounter("tls.sentByteCount")
		}).Should(BeEquivalentTo(sentLength))
	})

	It("counts the number of messages sent", func() {
		client.WriteOutput.sentLength <- len(message)
		client.WriteOutput.err <- nil

		err := tlsWrapper.Write(client, message)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() uint64 {
			return sender.GetCounter("tls.sentMessageCount")
		}).Should(BeEquivalentTo(1))
	})

	It("writes a message", func() {
		client.WriteOutput.sentLength <- len(message)
		client.WriteOutput.err <- nil

		err := tlsWrapper.Write(client, message)
		Expect(err).NotTo(HaveOccurred())

		Eventually(client.WriteCalled).Should(HaveLen(1))
		Eventually(client.WriteInput.message).Should(Receive(Equal(message)))
		Consistently(client.CloseCalled).ShouldNot(Receive())
	})

	Context("write returns an error", func() {
		BeforeEach(func() {
			client.WriteOutput.err <- errors.New("failure")
			client.WriteOutput.sentLength <- 0
		})

		It("returns an error and *only* increments sendErrorCount", func() {
			err := tlsWrapper.Write(client, message)
			Expect(err).To(HaveOccurred())

			Consistently(func() uint64 { return sender.GetCounter("tls.sentMessageCount") }).Should(BeZero())
			Consistently(func() uint64 { return sender.GetCounter("tls.sentByteCount") }).Should(BeZero())
			Eventually(func() uint64 { return sender.GetCounter("tls.sendErrorCount") }).Should(BeEquivalentTo(1))
		})

		It("closes the client", func() {
			err := tlsWrapper.Write(client, message)
			Expect(err).To(HaveOccurred())

			Eventually(client.CloseCalled).Should(Receive())
		})
	})
})
