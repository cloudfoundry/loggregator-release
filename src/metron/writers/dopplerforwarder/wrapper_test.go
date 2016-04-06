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

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Wrapper", func() {
	var (
		logger *gosteno.Logger
		sender *fake.FakeMetricSender

		client  *mockClient
		message []byte

		protocol string
		wrapper  *dopplerforwarder.Wrapper
	)

	BeforeEach(func() {
		logger = loggertesthelper.Logger()
		sender = fake.NewFakeMetricSender()
		metrics.Initialize(sender, metricbatcher.New(sender, time.Millisecond*10))

		client = newMockClient()
		envelope := &events.Envelope{
			Origin:     proto.String("fake-origin-1"),
			EventType:  events.Envelope_LogMessage.Enum(),
			LogMessage: factories.NewLogMessage(events.LogMessage_OUT, "message", "appid", "sourceType"),
		}
		var err error
		message, err = proto.Marshal(envelope)
		Expect(err).NotTo(HaveOccurred())

		protocol = ""
	})

	JustBeforeEach(func() {
		wrapper = dopplerforwarder.NewWrapper(logger, protocol)
	})

	Context("with a tcp wrapper", func() {
		BeforeEach(func() {
			protocol = "tcp"
		})

		It("counts the number of bytes sent", func() {
			// subtract a magic number three from the message length to make sure
			// we report the bytes sent by the client, not just the len of the
			// message given to it
			sentLength := len(message) - 3
			client.WriteOutput.sentLength <- sentLength
			client.WriteOutput.err <- nil

			err := wrapper.Write(client, message)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() uint64 {
				return sender.GetCounter("tcp.sentByteCount")
			}).Should(BeEquivalentTo(sentLength))
		})

		It("counts the number of messages sent", func() {
			client.WriteOutput.sentLength <- len(message)
			client.WriteOutput.err <- nil
			err := wrapper.Write(client, message)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() uint64 {
				return sender.GetCounter("tcp.sentMessageCount")
			}).Should(BeEquivalentTo(1))

			client.WriteOutput.sentLength <- len(message)
			client.WriteOutput.err <- nil
			err = wrapper.Write(client, message)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() uint64 {
				return sender.GetCounter("tcp.sentMessageCount")
			}).Should(BeEquivalentTo(2))
		})

		Context("with a client that returns an error", func() {
			BeforeEach(func() {
				client.WriteOutput.err <- errors.New("failure")
				client.WriteOutput.sentLength <- 0
				client.CloseOutput.ret0 <- nil
			})

			It("returns an error and *only* increments sendErrorCount", func() {
				err := wrapper.Write(client, message)
				Expect(err).To(HaveOccurred())

				Consistently(func() uint64 {
					return sender.GetCounter("tcp.sentMessageCount") +
						sender.GetCounter("tcp.sentByteCount")
				}).Should(BeZero())
				Eventually(func() uint64 {
					return sender.GetCounter("tcp.sendErrorCount")
				}).Should(BeEquivalentTo(1))
			})

			It("closes the client", func() {
				err := wrapper.Write(client, message)
				Expect(err).To(HaveOccurred())

				Eventually(client.CloseCalled).Should(Receive())
			})
		})
	})

	Context("with a tls wrapper", func() {
		BeforeEach(func() {
			protocol = "tls"
		})

		It("emits metrics with tcp prefix", func() {
			client.WriteOutput.sentLength <- len(message)
			client.WriteOutput.err <- nil
			err := wrapper.Write(client, message)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() uint64 {
				return sender.GetCounter("tls.sentMessageCount")
			}).Should(BeEquivalentTo(1))
			Eventually(func() uint64 {
				return sender.GetCounter("tls.sentByteCount")
			}).Should(BeEquivalentTo(len(message)))

			client.WriteOutput.sentLength <- 0
			client.WriteOutput.err <- errors.New("failure")
			client.CloseOutput.ret0 <- nil
			err = wrapper.Write(client, message)
			Expect(err).To(HaveOccurred())

			Eventually(func() uint64 {
				return sender.GetCounter("tls.sendErrorCount")
			}).Should(BeEquivalentTo(1))
		})
	})
})
