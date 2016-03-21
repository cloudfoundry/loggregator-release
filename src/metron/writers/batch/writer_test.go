package batch_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"metron/writers/batch"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
)

var bufferSize uint64

var _ = Describe("Batch Writer", func() {
	var (
		byteWriter      *mockWriter
		messageBytes    []byte
		prefixedMessage []byte
		batcher         *batch.Writer
		flushTimeout    time.Duration

		metricsSender  *fake.FakeMetricSender
		constructorErr error
	)

	BeforeEach(func() {
		metricsSender = fake.NewFakeMetricSender()
		metrics.Initialize(metricsSender, metricbatcher.New(metricsSender, time.Millisecond*10))
		byteWriter = newMockWriter()
		close(byteWriter.WriteOutput.Err)
		messageBytes = []byte("this is a log message")
		flushTimeout = time.Second / 2
		bufferSize = 1024

		// zero out the values that are assigned in the JustBeforeEach
		prefixedMessage = nil
		batcher = nil
		constructorErr = nil
	})

	JustBeforeEach(func() {
		prefixedMessage = prefixWithLength(messageBytes)
		logger := loggertesthelper.Logger()
		batcher, constructorErr = batch.NewWriter(byteWriter, bufferSize, flushTimeout, logger)
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
			flushTimeout = 0
		})

		It("doesn't flush on startup", func() {
			Consistently(byteWriter.WriteInput.P).ShouldNot(Receive())
		})
	})

	Context("messages larger than buffer size", func() {
		BeforeEach(func() {
			for uint64(len(messageBytes)) < bufferSize {
				messageBytes = append(messageBytes, messageBytes...)
			}
		})

		It("writes message to client", func() {
			byteWriter.WriteOutput.N <- len(prefixedMessage)
			bytesWritten, err := batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())
			Expect(bytesWritten).To(Equal(len(messageBytes) + 4))
			Eventually(byteWriter.WriteInput.P).Should(Receive(Equal(prefixedMessage)))
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
			Expect(byteWriter.WriteCalled).To(HaveLen(0))
		})

		It("flushes multiple messages within buffer when exceeding capacity", func() {
			close(byteWriter.WriteOutput.N)
			_, err := batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())

			_, err = batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())
			Eventually(byteWriter.WriteInput.P).Should(Receive(Equal(twoMessageOutput)))
			Consistently(byteWriter.WriteInput.P).ShouldNot(Receive())
		})

		It("resets the buffer when buffer is flushed", func() {
			close(byteWriter.WriteOutput.N)
			_, err := batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())

			_, err = batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())
			Eventually(byteWriter.WriteInput.P).Should(Receive(Equal(twoMessageOutput)))

			_, err = batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())
			Expect(byteWriter.WriteInput.P).To(HaveLen(0))
		})

		Context("the byte writer errors once", func() {
			BeforeEach(func() {
				byteWriter.WriteOutput.Err = make(chan error, 10)

				byteWriter.WriteOutput.N <- 0
				byteWriter.WriteOutput.Err <- errors.New("boom")

				byteWriter.WriteOutput.N <- len(prefixedMessage)
				close(byteWriter.WriteOutput.Err)
			})

			It("retries", func() {
				bytesWritten, err := batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
				Expect(bytesWritten).To(BeEquivalentTo(len(prefixedMessage)))

				Eventually(byteWriter.WriteInput.P).Should(Receive(Equal(prefixedMessage)))
				Eventually(byteWriter.WriteInput.P).Should(Receive(Equal(prefixedMessage)))
				Consistently(byteWriter.WriteInput.P).ShouldNot(Receive())
			})

			It("increments a retryCount metric", func() {
				bytesWritten, err := batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
				Expect(bytesWritten).To(BeEquivalentTo(len(prefixedMessage)))

				Eventually(func() uint64 {
					return metricsSender.GetCounter("DopplerForwarder.retryCount")
				}, 2).Should(BeEquivalentTo(1))
			})
		})

		Context("the byte writer repeatedly errors", func() {
			BeforeEach(func() {
				close(byteWriter.WriteOutput.N)
				byteWriter.WriteOutput.Err = make(chan error, 100)

				infinity := 50 // Close enough.
				for i := 0; i < infinity; i++ {
					byteWriter.WriteOutput.Err <- errors.New("To INFINITY (but not beyond)")
				}
			})

			It("sends a dropped message count", func() {
				// first write goes into the buffer
				_, err := batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())

				// forces a flush
				_, err = batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())

				droppedCount := func() uint64 {
					return metricsSender.GetCounter("MessageBuffer.droppedMessageCount")
				}
				Eventually(droppedCount).Should(BeEquivalentTo(2))
				Consistently(func() uint64 {
					return metricsSender.GetCounter("DopplerForwarder.sentMessages")
				}).Should(BeEquivalentTo(0))

				// The buffer should have been reset, so the next write will save
				// to the buffer.
				_, err = batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
				Consistently(droppedCount).Should(BeEquivalentTo(2))
			})
		})
	})

	Context("with flushTimeout", func() {
		BeforeEach(func() {
			// Use a buffer size large enough that no flush happens
			// due to a full buffer.
			bufferSize = 10000
			flushTimeout = 100 * time.Millisecond
		})

		It("flushes buffer after specific flushTimeout", func() {
			close(byteWriter.WriteOutput.N)
			_, err := batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())

			Eventually(byteWriter.WriteInput.P).Should(Receive(Equal(prefixedMessage)))

			_, err = batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())

			Eventually(byteWriter.WriteInput.P).Should(Receive(Equal(prefixedMessage)))
		})

		It("doesn't reset the timer every write during consistent low load", func() {
			// Ensure that write is called more frequently than the flushTimeout
			ticker := time.NewTicker(75 * time.Millisecond)
			defer ticker.Stop()
			go func() {
				for range ticker.C {
					batcher.Write([]byte("a"))
				}
			}()

			Eventually(byteWriter.WriteInput.P).Should(Receive())
		})

		It("doesn't flush write if there are no messages", func() {
			Consistently(byteWriter.WriteCalled, 110*time.Millisecond).ShouldNot(Receive())
		}, 2)

		It("sends a sentMessages metric on flush", func() {
			close(byteWriter.WriteOutput.N)
			for i := 0; i < 3; i++ {
				_, err := batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
			}
			sentMessages := func() uint64 { return metricsSender.GetCounter("DopplerForwarder.sentMessages") }
			Consistently(sentMessages, 90*time.Millisecond).Should(BeEquivalentTo(0))

			Eventually(byteWriter.WriteInput.P).Should(Receive())
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
