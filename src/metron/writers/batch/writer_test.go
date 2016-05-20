package batch_test

import (
	. "matchers"

	"bytes"
	"encoding/binary"
	"errors"
	"metron/writers/batch"
	"time"

	. "github.com/apoydence/eachers"
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

var bufferSize uint64

var _ = Describe("Batch Writer", func() {

	var (
		byteWriter      *mockBatchChainByteWriter
		messageBytes    []byte
		prefixedMessage []byte
		batcher         *batch.Writer
		timeout         time.Duration
		logger          *gosteno.Logger
		sender          *fake.FakeMetricSender
		mockBatcher     *mockMetricBatcher
		constructorErr  error
	)

	BeforeEach(func() {
		sender = fake.NewFakeMetricSender()
		mockBatcher = newMockMetricBatcher()
		metrics.Initialize(sender, mockBatcher)
		byteWriter = newMockBatchChainByteWriter()
		close(byteWriter.WriteOutput.Err)
		messageBytes = []byte("this is a log message")
		timeout = time.Second / 2
		bufferSize = 1024
		logger = loggertesthelper.Logger()

		// zero out the values that are assigned in the JustBeforeEach
		prefixedMessage = nil
		batcher = nil
		constructorErr = nil
	})

	JustBeforeEach(func() {
		prefixedMessage = prefixWithLength(messageBytes)
		batcher, constructorErr = batch.NewWriter(byteWriter, bufferSize, timeout, logger)
	})

	AfterEach(func() {
		if batcher != nil {
			batcher.Stop()
		}
	})

	Context("very small buffer size", func() {
		BeforeEach(func() {
			bufferSize = 10
		})

		It("errors in the constructor", func() {
			Expect(constructorErr).To(HaveOccurred())
			Expect(constructorErr.Error()).To(Equal("batch.Writer requires a buffer of at least 1024 bytes"))
		})
	})

	Context("short flush duration", func() {
		BeforeEach(func() {
			timeout = 0
		})

		It("doesn't flush on startup", func() {
			Consistently(byteWriter.WriteInput.Message).ShouldNot(Receive())
		})
	})

	Context("messages larger than buffer size", func() {

		BeforeEach(func() {
			for uint64(len(messageBytes)) < bufferSize {
				messageBytes = append(messageBytes, messageBytes...)
			}
		})

		It("writes message to client", func() {
			byteWriter.WriteOutput.SentLength <- len(prefixedMessage)
			bytesWritten, err := batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())
			Expect(bytesWritten).To(Equal(len(messageBytes)))
			Expect(byteWriter.WriteInput.Message).To(Receive(Equal(prefixedMessage)))
		})

		It("sends a sentMessages metric", func() {
			close(byteWriter.WriteOutput.SentLength)
			_, err := batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())

			Eventually(byteWriter.WriteInput.Message).Should(Receive(Equal(prefixedMessage)))
			Eventually(mockBatcher.BatchAddCounterInput).Should(BeCalled(
				With("DopplerForwarder.sentMessages"),
			))
		})

		It("passes through any chainers", func() {
			close(byteWriter.WriteOutput.SentLength)
			chainers := []metricbatcher.BatchCounterChainer{
				newMockBatchCounterChainer(),
				newMockBatchCounterChainer(),
			}
			_, err := batcher.Write(messageBytes, chainers...)
			Expect(err).ToNot(HaveOccurred())

			Eventually(byteWriter.WriteInput).Should(BeCalled(
				With(prefixedMessage, chainers),
			))

			newChainers := []metricbatcher.BatchCounterChainer{
				newMockBatchCounterChainer(),
			}
			_, err = batcher.Write(messageBytes, newChainers...)
			Expect(err).ToNot(HaveOccurred())

			Eventually(byteWriter.WriteInput).Should(BeCalled(
				With(prefixedMessage, newChainers),
			))
		})

		Context("the weighted writer errors once", func() {
			BeforeEach(func() {
				byteWriter.WriteOutput.SentLength <- 0
				byteWriter.WriteOutput.Err = make(chan error, 1)
				byteWriter.WriteOutput.Err <- errors.New("boom")
			})

			JustBeforeEach(func() {
				byteWriter.WriteOutput.SentLength <- len(prefixedMessage)
				close(byteWriter.WriteOutput.Err)
			})

			It("retries", func() {
				bytesWritten, err := batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
				Expect(bytesWritten).To(BeEquivalentTo(len(messageBytes)))

				Eventually(byteWriter.WriteInput.Message).Should(Receive(Equal(prefixedMessage)))
				Eventually(byteWriter.WriteInput.Message).Should(Receive(Equal(prefixedMessage)))
				Consistently(byteWriter.WriteInput.Message).ShouldNot(Receive())
			})

			It("increments a retryCount metric", func() {
				bytesWritten, err := batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
				Expect(bytesWritten).To(BeEquivalentTo(len(messageBytes)))

				Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
					With("DopplerForwarder.retryCount"),
				))
			})
		})

		Context("the weighted writer repeatedly errors", func() {
			BeforeEach(func() {
				close(byteWriter.WriteOutput.SentLength)

				// close enough
				infinity := 10
				byteWriter.WriteOutput.Err = make(chan error, infinity)
				for i := 0; i < infinity; i++ {
					byteWriter.WriteOutput.Err <- errors.New("To INFINITY (but not beyond)")
				}
			})

			It("sends a dropped message count", func() {
				bytesWritten, err := batcher.Write(messageBytes)
				Expect(err).To(HaveOccurred())
				Expect(bytesWritten).To(BeEquivalentTo(0))

				Eventually(mockBatcher.BatchAddCounterInput).Should(BeCalled(
					With("MessageBuffer.droppedMessageCount"),
				))
				Consistently(func() uint64 { return sender.GetCounter("DopplerForwarder.sentMessages") }).Should(BeEquivalentTo(0))
			})

			It("writes a log containing dropped message count", func() {
				bytesWritten, err := batcher.Write(messageBytes)
				Expect(err).To(HaveOccurred())
				Expect(bytesWritten).To(BeEquivalentTo(0))

				expected := &events.Envelope{
					EventType: events.Envelope_LogMessage.Enum(),
					LogMessage: &events.LogMessage{
						MessageType: events.LogMessage_ERR.Enum(),
						AppId:       proto.String(envelope_extensions.SystemAppId),
						Message:     []byte("Dropped 1 message(s) from MetronAgent to Doppler"),
					},
				}
				Eventually(byteWriter.WriteInput.Message, 2).Should(ReceivePrefixedEnvelope(MatchSpecifiedContents(expected)))
				Consistently(byteWriter.WriteInput.Message).ShouldNot(Receive())
			})
		})
	})

	Context("message smaller than buffer size", func() {
		var twoMessageOutput []byte

		BeforeEach(func() {
			for uint64(len(messageBytes)) < bufferSize/2 {
				messageBytes = append(messageBytes, messageBytes...)
			}
		})

		JustBeforeEach(func() {
			twoMessageOutput = append(prefixedMessage, prefixedMessage...)
		})

		It("batches multiple messages within buffer without flushing", func() {
			bytesWritten, err := batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())
			Expect(bytesWritten).To(Equal(len(messageBytes)))
			Expect(byteWriter.WriteCalled).To(HaveLen(0))
		})

		It("flushes multiple messages within buffer when exceeding capacity", func() {
			close(byteWriter.WriteOutput.SentLength)
			_, err := batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())

			_, err = batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())
			Expect(byteWriter.WriteInput.Message).To(Receive(Equal(twoMessageOutput)))
			Consistently(byteWriter.WriteInput.Message).ShouldNot(Receive())
		})

		It("flushes multiple chainers when exceeding capacity", func() {
			close(byteWriter.WriteOutput.SentLength)
			chainers := []metricbatcher.BatchCounterChainer{
				newMockBatchCounterChainer(),
				newMockBatchCounterChainer(),
			}
			_, err := batcher.Write(messageBytes, chainers[0])
			Expect(err).ToNot(HaveOccurred())
			_, err = batcher.Write(messageBytes, chainers[1])

			Eventually(byteWriter.WriteInput).Should(BeCalled(
				With(twoMessageOutput, chainers),
			))
		})

		It("resets the buffer when buffer is flushed", func() {
			close(byteWriter.WriteOutput.SentLength)
			_, err := batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())

			_, err = batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())
			Expect(byteWriter.WriteInput.Message).To(Receive(Equal(twoMessageOutput)))

			_, err = batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())
			Expect(byteWriter.WriteInput.Message).To(HaveLen(0))
		})

		Context("the weighted writer errors once", func() {
			BeforeEach(func() {
				byteWriter.WriteOutput.SentLength <- 0
				byteWriter.WriteOutput.Err = make(chan error, 10)
				byteWriter.WriteOutput.Err <- errors.New("boom")
				byteWriter.WriteOutput.SentLength <- len(prefixedMessage)
				close(byteWriter.WriteOutput.Err)
			})

			It("retries", func() {
				bytesWritten, err := batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
				Expect(bytesWritten).To(BeEquivalentTo(len(messageBytes)))

				Eventually(byteWriter.WriteInput.Message).Should(Receive(Equal(prefixedMessage)))
				Consistently(byteWriter.WriteInput.Message, 0.4).ShouldNot(Receive())
				Eventually(byteWriter.WriteInput.Message).Should(Receive(Equal(prefixedMessage)))
				Consistently(byteWriter.WriteInput.Message).ShouldNot(Receive())
			})

			It("increments a retryCount metric", func() {
				bytesWritten, err := batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
				Expect(bytesWritten).To(BeEquivalentTo(len(messageBytes)))

				Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
					With("DopplerForwarder.retryCount"),
				))
			})
		})

		Context("the weighted writer repeatedly errors", func() {
			BeforeEach(func() {
				close(byteWriter.WriteOutput.SentLength)
				byteWriter.WriteOutput.Err = make(chan error, 100)
				// Close enough.
				infinity := 50
				for i := 0; i < infinity; i++ {
					byteWriter.WriteOutput.Err <- errors.New("To INFINITY (but not beyond)")
				}
			})

			It("sends a dropped message count", func() {
				_, err := batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
				_, err = batcher.Write(messageBytes)
				Expect(err).To(HaveOccurred())

				Eventually(mockBatcher.BatchAddCounterInput).Should(BeCalled(
					With("MessageBuffer.droppedMessageCount", uint64(2)),
				))
				Consistently(mockBatcher.BatchAddCounterInput).ShouldNot(BeCalled(
					With("DopplerForwarder.sentMessages"),
				))

				// The buffer should have been reset, so the next write will save
				// to the buffer.
				_, err = batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
				Consistently(mockBatcher.BatchAddCounterInput).ShouldNot(BeCalled())
			})

			It("writes a log containing dropped message count", func() {
				_, err := batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
				bytesWritten, err := batcher.Write(messageBytes)
				Expect(err).To(HaveOccurred())
				Expect(bytesWritten).To(BeEquivalentTo(0))

				expected := &events.Envelope{
					EventType: events.Envelope_LogMessage.Enum(),
					LogMessage: &events.LogMessage{
						MessageType: events.LogMessage_ERR.Enum(),
						AppId:       proto.String(envelope_extensions.SystemAppId),
						Message:     []byte("Dropped 2 message(s) from MetronAgent to Doppler"),
					},
				}
				Eventually(byteWriter.WriteInput.Message, 2).Should(ReceiveEnvelope(MatchSpecifiedContents(expected)))
				Consistently(byteWriter.WriteInput.Message).ShouldNot(Receive())
			})

			It("adds the dropped message counts before accepting any more messages", func() {
				bufferFiller := make([]byte, bufferSize-10)
				_, err := batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
				bytesWritten, err := batcher.Write(messageBytes)
				Expect(err).To(HaveOccurred())
				Expect(bytesWritten).To(BeEquivalentTo(0))

				// If the message about dropped messages was already added, the bufferFiller
				// will cause a flush and error.
				_, err = batcher.Write(bufferFiller)
				Expect(err).To(HaveOccurred())

				expected := &events.Envelope{
					EventType: events.Envelope_LogMessage.Enum(),
					LogMessage: &events.LogMessage{
						MessageType: events.LogMessage_ERR.Enum(),
						AppId:       proto.String(envelope_extensions.SystemAppId),
						Message:     []byte("Dropped 3 message(s) from MetronAgent to Doppler"),
					},
				}
				Eventually(byteWriter.WriteInput.Message, 2).Should(ReceiveEnvelope(MatchSpecifiedContents(expected)))
				Consistently(byteWriter.WriteInput.Message).ShouldNot(Receive())
			})

			Context("after the byte writer resumes normal function", func() {
				JustBeforeEach(func() {
					// Overflowing buffer to get it to drop messages
					_, err := batcher.Write(messageBytes)
					Expect(err).ToNot(HaveOccurred())
					_, err = batcher.Write(messageBytes)
					Expect(err).To(HaveOccurred())
					Eventually(mockBatcher.BatchAddCounterInput).Should(BeCalled(
						With("MessageBuffer.droppedMessageCount", uint64(2)),
					))

					byteWriter.WriteOutput.Err = make(chan error, 100)
					byteWriter.WriteOutput.SentLength = make(chan int, 100)
				})

				It("resets the dropped count after a successful flush", func() {
					byteWriter.WriteOutput.Err <- nil
					byteWriter.WriteOutput.SentLength <- len(prefixedMessage)

					_, err := batcher.Write(messageBytes)
					Expect(err).ToNot(HaveOccurred())
					_, err = batcher.Write(messageBytes)
					Expect(err).ToNot(HaveOccurred())

					Eventually(mockBatcher.BatchAddCounterInput).Should(BeCalled(
						With("DopplerForwarder.sentMessages", uint64(2)),
					))

					close(byteWriter.WriteOutput.SentLength)
					infinity := 50
					for i := 0; i < infinity; i++ {
						byteWriter.WriteOutput.Err <- errors.New("To INFINITY (but not beyond)")
					}

					_, err = batcher.Write(messageBytes)
					Expect(err).ToNot(HaveOccurred())
					_, err = batcher.Write(messageBytes)
					Expect(err).To(HaveOccurred())

					expected := &events.Envelope{
						EventType: events.Envelope_LogMessage.Enum(),
						LogMessage: &events.LogMessage{
							MessageType: events.LogMessage_ERR.Enum(),
							AppId:       proto.String(envelope_extensions.SystemAppId),
							Message:     []byte("Dropped 2 message(s) from MetronAgent to Doppler"),
						},
					}
					Eventually(byteWriter.WriteInput.Message, 2).Should(ReceiveEnvelope(MatchSpecifiedContents(expected)))
					Consistently(byteWriter.WriteInput.Message).ShouldNot(Receive())
				})
			})
		})
	})

	Context("with timeout", func() {

		BeforeEach(func() {
			// Use a buffer size large enough that no flush happens
			// due to a full buffer.
			bufferSize = 10000
			timeout = 100 * time.Millisecond
		})

		It("flushes buffer after specific timeout", func() {
			close(byteWriter.WriteOutput.SentLength)
			_, err := batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())

			Eventually(byteWriter.WriteInput.Message).Should(Receive(Equal(prefixedMessage)))

			_, err = batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())

			Eventually(byteWriter.WriteInput.Message).Should(Receive(Equal(prefixedMessage)))

		})

		It("doesn't reset the timer every write during consistent low load", func() {
			// Ensure that write is called more frequently than the timeout
			ticker := time.NewTicker(75 * time.Millisecond)
			defer ticker.Stop()
			go func() {
				for range ticker.C {
					batcher.Write([]byte("a"))
				}
			}()

			Eventually(byteWriter.WriteInput.Message).Should(Receive())
			close(byteWriter.WriteOutput.SentLength)
		})

		It("sends a sentMessages metric on flush", func() {
			close(byteWriter.WriteOutput.SentLength)
			for i := 0; i < 3; i++ {
				_, err := batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
			}
			Consistently(mockBatcher.BatchAddCounterInput).ShouldNot(BeCalled(
				With("DopplerForwarder.sentMessages"),
			))

			Eventually(byteWriter.WriteInput.Message).Should(Receive())
			Eventually(mockBatcher.BatchAddCounterInput).Should(BeCalled(
				With("DopplerForwarder.sentMessages", uint64(3)),
			))
		})
	})
})

func prefixWithLength(message []byte) []byte {
	buffer := bytes.NewBuffer(make([]byte, 0, bufferSize*2))
	binary.Write(buffer, binary.LittleEndian, uint32(len(message)))
	buffer.Write(message)
	return buffer.Bytes()
}
