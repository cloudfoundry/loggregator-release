package batch_test

import (
	"errors"
	"metron/writers/batch"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/dropsonde/envelope_extensions"
	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

var _ = Describe("AsyncRetryWriter", func() {
	var (
		mockWriter, mockErrWriter *mockWriter
		logger                    *gosteno.Logger

		asyncWriter *batch.AsyncRetryWriter

		message      []byte
		retryCount   int
		messageCount uint64

		metricsSender *fake.FakeMetricSender
	)

	BeforeEach(func() {
		mockWriter = newMockWriter()
		mockErrWriter = newMockWriter()
		logger = loggertesthelper.Logger()

		asyncWriter = batch.NewAsyncRetryWriter(mockWriter, mockErrWriter, logger)

		message = []byte("hello world")
		retryCount = 0
		messageCount = 0

		metricsSender = fake.NewFakeMetricSender()
		metrics.Initialize(metricsSender, metricbatcher.New(metricsSender, time.Millisecond*10))
	})

	JustBeforeEach(func(done Done) {
		defer close(done)
		asyncWriter.AsyncWrite(message, retryCount, messageCount)
	})

	Describe("AsyncWrite", func() {
		It("does not block if the writer it was given blocks", func() {
			Eventually(mockWriter.WriteInput.P).Should(Receive(Equal([]byte(`hello world`))))
		})

		Context("with a writer that immediately succeeds", func() {
			BeforeEach(func() {
				mockWriter.WriteOutput.N <- 11
				mockWriter.WriteOutput.Err <- nil
				messageCount = 1
			})
			It("writes into the writer it was given", func() {
				Eventually(mockWriter.WriteInput.P).Should(Receive(Equal([]byte(`hello world`))))
			})
		})

		Context("with a writer that errors out a few times", func() {
			BeforeEach(func() {
				mockWriter.WriteOutput.N <- 0
				mockWriter.WriteOutput.Err <- errors.New("write failed")
				mockWriter.WriteOutput.N <- 0
				mockWriter.WriteOutput.Err <- errors.New("write failed")
				mockWriter.WriteOutput.N <- 0
				mockWriter.WriteOutput.Err <- errors.New("write failed")
				mockErrWriter.WriteOutput.N <- 0
				mockErrWriter.WriteOutput.Err <- errors.New("write failed")

				retryCount = 2
				messageCount = 1
			})

			It("retries", func() {
				Eventually(mockWriter.WriteInput.P).Should(Receive(Equal([]byte("hello world"))))
				Eventually(mockWriter.WriteInput.P).Should(Receive(Equal([]byte("hello world"))))
				Eventually(mockWriter.WriteInput.P).Should(Receive(Equal([]byte("hello world"))))
			})

			It("emits a metric every time it retries", func() {
				Eventually(func() uint64 {
					return metricsSender.GetCounter("DopplerForwarder.retryCount")
				}).Should(BeEquivalentTo(2))
			})
		})

		Context("with a writer that errors but then succeeds", func() {
			BeforeEach(func() {
				mockWriter.WriteOutput.N <- 0
				mockWriter.WriteOutput.Err <- errors.New("write failed")
				mockWriter.WriteOutput.N <- 11
				mockWriter.WriteOutput.Err <- nil
				mockWriter.WriteOutput.N <- 11
				mockWriter.WriteOutput.Err <- nil

				retryCount = 2
				messageCount = 3

				asyncWriter.AsyncWrite([]byte("hello world"), retryCount, messageCount)
			})

			It("increments sentMessages metric when messages are successfully sent", func() {
				Eventually(func() uint64 {
					return metricsSender.GetCounter("DopplerForwarder.sentMessages")
				}).Should(BeEquivalentTo(6))
			})
		})

		Context("with a write that fails to send", func() {
			BeforeEach(func() {
				mockWriter.WriteOutput.N <- 0
				mockWriter.WriteOutput.Err <- errors.New("write failed")
				mockWriter.WriteOutput.N <- 0
				mockWriter.WriteOutput.Err <- errors.New("write failed")
				mockErrWriter.WriteOutput.N <- 0
				mockErrWriter.WriteOutput.Err <- errors.New("write failed")

				retryCount = 1
				messageCount = 4
			})

			It("increments droppedMessageCount metric", func() {
				Eventually(func() uint64 {
					return metricsSender.GetCounter("MessageBuffer.droppedMessageCount")
				}).Should(BeEquivalentTo(4))
				Consistently(func() uint64 {
					return metricsSender.GetCounter("DopplerForwarder.sentMessages")
				}).Should(BeEquivalentTo(0))
			})

			It("writes a dropped message log in to the err writer", func() {
				var bytes []byte
				Eventually(mockErrWriter.WriteInput.P).Should(Receive(&bytes))

				env := &events.Envelope{}
				err := proto.Unmarshal(bytes, env)
				Expect(err).ToNot(HaveOccurred())

				expectedMessage := []byte("Dropped 4 message(s) from MetronAgent to Doppler")
				logMsg := env.GetLogMessage()
				Expect(logMsg.GetMessage()).To(Equal(expectedMessage))
				Expect(logMsg.GetMessageType()).To(Equal(events.LogMessage_ERR))
				Expect(logMsg.GetAppId()).To(Equal(envelope_extensions.SystemAppId))
			})
		})
	})
})
