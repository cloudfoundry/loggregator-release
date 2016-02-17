package batch_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"metron/writers/batch"
	"time"

	. "matchers"

	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/dropsonde/envelope_extensions"
	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
)

var bufferSize uint64

var _ = Describe("Batch Writer", func() {

	var (
		weightedWriter  *mockWeightedWriter
		messageBytes    []byte
		prefixedMessage []byte
		batcher         *batch.Writer
		timeout         time.Duration
		logger          *gosteno.Logger
		sender          *fake.FakeMetricSender
		constructorErr  error
	)

	BeforeEach(func() {
		sender = fake.NewFakeMetricSender()
		metrics.Initialize(sender, metricbatcher.New(sender, time.Millisecond*10))
		weightedWriter = newMockWeightedWriter()
		close(weightedWriter.WriteOutput.err)
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
		batcher, constructorErr = batch.NewWriter(weightedWriter, bufferSize, timeout, logger)
	})

	AfterEach(func() {
		if batcher != nil {
			batcher.Stop()
		}
	})

	Describe("Weight", func() {
		It("returns the weight of the output", func() {
			weightedWriter.WeightOutput.ret0 <- 111
			Expect(batcher.Weight()).To(BeEquivalentTo(111))
		})
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
			Consistently(weightedWriter.WriteInput.bytes).ShouldNot(Receive())
		})
	})

	Context("messages larger than buffer size", func() {

		BeforeEach(func() {
			for uint64(len(messageBytes)) < bufferSize {
				messageBytes = append(messageBytes, messageBytes...)
			}
		})

		It("writes message to client", func() {
			weightedWriter.WriteOutput.sentLength <- len(prefixedMessage)
			bytesWritten, err := batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())
			Expect(bytesWritten).To(Equal(len(messageBytes) + 4))
			Expect(weightedWriter.WriteInput.bytes).To(Receive(Equal(prefixedMessage)))
		})

		It("sends a sentMessages metric", func() {
			close(weightedWriter.WriteOutput.sentLength)
			_, err := batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())

			Eventually(weightedWriter.WriteInput.bytes).Should(Receive(Equal(prefixedMessage)))
			Eventually(func() uint64 { return sender.GetCounter("DopplerForwarder.sentMessages") }).Should(BeEquivalentTo(1))
		})

		Context("the weighted writer errors once", func() {
			BeforeEach(func() {
				weightedWriter.WriteOutput.sentLength <- 0
				weightedWriter.WriteOutput.err = make(chan error, 1)
				weightedWriter.WriteOutput.err <- errors.New("boom")
			})

			JustBeforeEach(func() {
				weightedWriter.WriteOutput.sentLength <- len(prefixedMessage)
				close(weightedWriter.WriteOutput.err)
			})

			It("retries", func() {
				bytesWritten, err := batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
				Expect(bytesWritten).To(BeEquivalentTo(len(prefixedMessage)))

				Eventually(weightedWriter.WriteInput.bytes).Should(Receive(Equal(prefixedMessage)))
				Eventually(weightedWriter.WriteInput.bytes).Should(Receive(Equal(prefixedMessage)))
				Consistently(weightedWriter.WriteInput.bytes).ShouldNot(Receive())
			})

			It("increments a retryCount metric", func() {
				bytesWritten, err := batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
				Expect(bytesWritten).To(BeEquivalentTo(len(prefixedMessage)))

				Eventually(func() uint64 { return sender.GetCounter("DopplerForwarder.retryCount") }).Should(BeEquivalentTo(1))
			})
		})

		Context("the weighted writer repeatedly errors", func() {
			BeforeEach(func() {
				close(weightedWriter.WriteOutput.sentLength)

				// close enough
				infinity := 10
				weightedWriter.WriteOutput.err = make(chan error, infinity)
				for i := 0; i < infinity; i++ {
					weightedWriter.WriteOutput.err <- errors.New("To INFINITY (but not beyond)")
				}
			})

			It("sends a dropped message count", func() {
				bytesWritten, err := batcher.Write(messageBytes)
				Expect(err).To(HaveOccurred())
				Expect(bytesWritten).To(BeEquivalentTo(0))

				Eventually(func() uint64 { return sender.GetCounter("MessageBuffer.droppedMessageCount") }).Should(BeEquivalentTo(1))
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
				Eventually(weightedWriter.WriteInput.bytes, 2).Should(ReceiveEnvelope(MatchSpecifiedContents(expected)))
				Consistently(weightedWriter.WriteInput.bytes).ShouldNot(Receive())
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
			Expect(bytesWritten).To(Equal(len(messageBytes) + 4))
			Expect(weightedWriter.WriteCalled).To(HaveLen(0))
		})

		It("flushes multiple messages within buffer when exceeding capacity", func() {
			close(weightedWriter.WriteOutput.sentLength)
			_, err := batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())

			_, err = batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())
			Expect(weightedWriter.WriteInput.bytes).To(Receive(Equal(twoMessageOutput)))
			Consistently(weightedWriter.WriteInput.bytes).ShouldNot(Receive())
		})

		It("resets the buffer when buffer is flushed", func() {
			close(weightedWriter.WriteOutput.sentLength)
			_, err := batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())

			_, err = batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())
			Expect(weightedWriter.WriteInput.bytes).To(Receive(Equal(twoMessageOutput)))

			_, err = batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())
			Expect(weightedWriter.WriteInput.bytes).To(HaveLen(0))
		})

		Context("the weighted writer errors once", func() {
			BeforeEach(func() {
				weightedWriter.WriteOutput.sentLength <- 0
				weightedWriter.WriteOutput.err = make(chan error, 10)
				weightedWriter.WriteOutput.err <- errors.New("boom")
				weightedWriter.WriteOutput.sentLength <- len(prefixedMessage)
				close(weightedWriter.WriteOutput.err)
			})

			It("retries", func() {
				bytesWritten, err := batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
				Expect(bytesWritten).To(BeEquivalentTo(len(prefixedMessage)))

				Eventually(weightedWriter.WriteInput.bytes).Should(Receive(Equal(prefixedMessage)))
				Consistently(weightedWriter.WriteInput.bytes, 0.4).ShouldNot(Receive())
				Eventually(weightedWriter.WriteInput.bytes).Should(Receive(Equal(prefixedMessage)))
				Consistently(weightedWriter.WriteInput.bytes).ShouldNot(Receive())
			})

			It("increments a retryCount metric", func() {
				bytesWritten, err := batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
				Expect(bytesWritten).To(BeEquivalentTo(len(prefixedMessage)))

				Eventually(func() uint64 { return sender.GetCounter("DopplerForwarder.retryCount") }, 2).Should(BeEquivalentTo(1))
			})
		})

		Context("the weighted writer repeatedly errors", func() {
			BeforeEach(func() {
				close(weightedWriter.WriteOutput.sentLength)
				weightedWriter.WriteOutput.err = make(chan error, 100)
				// Close enough.
				infinity := 50
				for i := 0; i < infinity; i++ {
					weightedWriter.WriteOutput.err <- errors.New("To INFINITY (but not beyond)")
				}
			})

			It("sends a dropped message count", func() {
				_, err := batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
				_, err = batcher.Write(messageBytes)
				Expect(err).To(HaveOccurred())

				droppedCount := func() uint64 { return sender.GetCounter("MessageBuffer.droppedMessageCount") }
				Eventually(droppedCount).Should(BeEquivalentTo(2))
				Eventually(func() uint64 { return sender.GetCounter("DopplerForwarder.sentMessages") }).Should(BeEquivalentTo(0))

				// The buffer should have been reset, so the next write will save
				// to the buffer.
				_, err = batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
				Consistently(droppedCount).Should(BeEquivalentTo(2))
			})

			It("writes a log containing dropped message count", func() {
				bytesWritten, err := batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
				bytesWritten, err = batcher.Write(messageBytes)
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
				Eventually(weightedWriter.WriteInput.bytes, 2).Should(ReceiveEnvelope(MatchSpecifiedContents(expected)))
				Consistently(weightedWriter.WriteInput.bytes).ShouldNot(Receive())
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
			close(weightedWriter.WriteOutput.sentLength)
			_, err := batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())

			Eventually(weightedWriter.WriteInput.bytes).Should(Receive(Equal(prefixedMessage)))

			_, err = batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())

			Eventually(weightedWriter.WriteInput.bytes).Should(Receive(Equal(prefixedMessage)))

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

			Eventually(weightedWriter.WriteInput.bytes).Should(Receive())
		})

		It("sends a sentMessages metric on flush", func() {
			close(weightedWriter.WriteOutput.sentLength)
			for i := 0; i < 3; i++ {
				_, err := batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
			}
			sentMessages := func() uint64 { return sender.GetCounter("DopplerForwarder.sentMessages") }
			Consistently(sentMessages, 90*time.Millisecond).Should(BeEquivalentTo(0))

			Eventually(weightedWriter.WriteInput.bytes).Should(Receive())
			Eventually(sentMessages).Should(BeEquivalentTo(3))
		})
	})
})

func prefixWithLength(message []byte) []byte {
	buffer := bytes.NewBuffer(make([]byte, 0, bufferSize*2))
	binary.Write(buffer, binary.LittleEndian, uint32(len(message)))
	buffer.Write(message)
	return buffer.Bytes()
}
