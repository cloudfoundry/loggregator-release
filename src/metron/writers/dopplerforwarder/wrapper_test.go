package dopplerforwarder_test

import (
	"errors"
	"metron/writers/dopplerforwarder"

	. "github.com/apoydence/eachers"
	"github.com/apoydence/eachers/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

var _ = Describe("Wrapper", func() {
	var (
		logger      *gosteno.Logger
		sender      *fake.FakeMetricSender
		mockBatcher *mockMetricBatcher

		client  *mockClient
		message []byte

		protocol string
		wrapper  *dopplerforwarder.Wrapper
	)

	BeforeEach(func() {
		logger = loggertesthelper.Logger()
		sender = fake.NewFakeMetricSender()
		mockBatcher = newMockMetricBatcher()
		metrics.Initialize(sender, mockBatcher)

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
			client.WriteOutput.SentLength <- sentLength
			client.WriteOutput.Err <- nil

			err := wrapper.Write(client, message)
			Expect(err).NotTo(HaveOccurred())

			Eventually(mockBatcher.BatchAddCounterInput).Should(BeCalled(
				With("tcp.sentByteCount", uint64(sentLength)),
			))
		})

		It("does not count the number of messages sent", func() {
			client.WriteOutput.SentLength <- len(message)
			client.WriteOutput.Err <- nil
			mockChainer := newMockBatchCounterChainer()
			testhelpers.AlwaysReturn(mockChainer.SetTagOutput, mockChainer)

			err := wrapper.Write(client, message, mockChainer)
			Expect(err).NotTo(HaveOccurred())

			Consistently(mockBatcher.BatchIncrementCounterInput).ShouldNot(BeCalled(
				With("tcp.sentMessageCount"),
			))
		})

		Context("with a client that returns an error", func() {
			BeforeEach(func() {
				client.WriteOutput.Err <- errors.New("failure")
				client.WriteOutput.SentLength <- 0
				client.CloseOutput.Ret0 <- nil
			})

			It("returns an error and *only* increments sendErrorCount", func() {
				err := wrapper.Write(client, message)
				Expect(err).To(HaveOccurred())

				var name string
				Eventually(mockBatcher.BatchIncrementCounterInput.Name).Should(Receive(&name))
				Expect(name).To(Equal("tcp.sendErrorCount"))
				Consistently(mockBatcher.BatchIncrementCounterInput).ShouldNot(BeCalled())
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

		It("emits sentByteCount metric", func() {
			client.WriteOutput.SentLength <- len(message)
			client.WriteOutput.Err <- nil
			err := wrapper.Write(client, message)
			Expect(err).ToNot(HaveOccurred())

			Eventually(mockBatcher.BatchAddCounterInput).Should(BeCalled(
				With("tls.sentByteCount", uint64(len(message))),
			))
		})

		It("emits sendErrorCount", func() {
			client.WriteOutput.SentLength <- 0
			client.WriteOutput.Err <- errors.New("failure")
			client.CloseOutput.Ret0 <- nil
			err := wrapper.Write(client, message)
			Expect(err).To(HaveOccurred())

			Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
				With("tls.sendErrorCount"),
			))
		})

		It("does not emit sentMessageCount metric", func() {
			client.WriteOutput.SentLength <- len(message)
			client.WriteOutput.Err <- nil
			err := wrapper.Write(client, message)
			Expect(err).ToNot(HaveOccurred())

			Consistently(mockBatcher.BatchIncrementCounterInput).ShouldNot(BeCalled(
				With("tls.sentMessageCount"),
			))
		})
	})
})
