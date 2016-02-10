package dopplerforwarder_test

import (
	"errors"
	"metron/writers/dopplerforwarder"
	"time"

	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	"bytes"
	"encoding/binary"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
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
		logger     *gosteno.Logger
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
		logger = loggertesthelper.Logger()
		tlsWrapper = dopplerforwarder.NewTLSWrapper(logger)

		var err error
		message, err = proto.Marshal(envelope)
		Expect(err).NotTo(HaveOccurred())

	})

	It("counts the number of bytes sent", func() {

		sentLength := len(message) - 3
		client.WriteOutput.sentLength <- 4
		client.WriteOutput.err <- nil

		client.WriteOutput.sentLength <- sentLength
		client.WriteOutput.err <- nil

		err := tlsWrapper.Write(client, message)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() uint64 {
			return sender.GetCounter("tls.sentByteCount")
		}).Should(BeEquivalentTo(sentLength + 4))
	})

	It("length prefixes the message bytes for single message", func() {
		client.WriteOutput.sentLength <- 4
		client.WriteOutput.err <- nil

		client.WriteOutput.sentLength <- len(message)
		client.WriteOutput.err <- nil

		err := tlsWrapper.Write(client, message)
		Expect(err).NotTo(HaveOccurred())

		expected := new(bytes.Buffer)
		binary.Write(expected, binary.LittleEndian, uint32(len(message)))

		Eventually(client.WriteInput.message).Should(Receive(Equal(expected.Bytes())))
		Eventually(client.WriteInput.message).Should(Receive(Equal(message)))
	})

	It("length prefixes the message bytes for multiple messages", func() {
		client.WriteOutput.sentLength <- 4
		client.WriteOutput.err <- nil

		client.WriteOutput.sentLength <- len(message)
		client.WriteOutput.err <- nil

		err := tlsWrapper.Write(client, message)
		Expect(err).NotTo(HaveOccurred())
		expected := new(bytes.Buffer)
		binary.Write(expected, binary.LittleEndian, uint32(len(message)))

		Eventually(client.WriteInput.message).Should(Receive(Equal(expected.Bytes())))
		Eventually(client.WriteInput.message).Should(Receive(Equal(message)))

		counterEnvelope := factories.NewCounterEvent("counter", 34)
		secondMessage, protoErr := proto.Marshal(counterEnvelope)
		Expect(protoErr).NotTo(HaveOccurred())

		client.WriteOutput.sentLength <- 4
		client.WriteOutput.err <- nil

		client.WriteOutput.sentLength <- len(secondMessage)
		client.WriteOutput.err <- nil

		secondWriteErr := tlsWrapper.Write(client, secondMessage)
		Expect(secondWriteErr).NotTo(HaveOccurred())

		secondExpectedMessage := new(bytes.Buffer)
		binary.Write(secondExpectedMessage, binary.LittleEndian, uint32(len(secondMessage)))

		Eventually(client.WriteInput.message).Should(Receive(Equal(secondExpectedMessage.Bytes())))
		Eventually(client.WriteInput.message).Should(Receive(Equal(secondMessage)))

	})

	It("counts the number of messages sent", func() {
		client.WriteOutput.sentLength <- 4
		client.WriteOutput.err <- nil

		client.WriteOutput.sentLength <- len(message)
		client.WriteOutput.err <- nil

		err := tlsWrapper.Write(client, message)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() uint64 {
			return sender.GetCounter("tls.sentMessageCount")
		}).Should(BeEquivalentTo(1))
	})

	Context("write returns an error", func() {
		BeforeEach(func() {
			client.WriteOutput.err <- errors.New("failure")
			client.WriteOutput.sentLength <- 0
			client.CloseOutput.ret0 <- nil
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
