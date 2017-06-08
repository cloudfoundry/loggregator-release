package v1_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"sync"
	"time"

	egress "code.cloudfoundry.org/loggregator/metron/internal/egress/v1"

	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"

	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var bufferSize uint64

var _ = Describe("Batch Writer", func() {

	var (
		byteWriter      *mockBatchChainByteWriter
		droppedCounter  *mockDroppedMessageCounter
		messageBytes    []byte
		prefixedMessage []byte
		batcher         *egress.Writer
		timeout         time.Duration
		sender          *fake.FakeMetricSender
		mockBatcher     *mockMetricBatcher
		constructorErr  error
	)

	BeforeEach(func() {
		sender = fake.NewFakeMetricSender()
		droppedCounter = newMockDroppedMessageCounter()
		mockBatcher = newMockMetricBatcher()
		metrics.Initialize(sender, mockBatcher)
		byteWriter = newMockBatchChainByteWriter()
		close(byteWriter.WriteOutput.Err)
		messageBytes = []byte("this is a log message")
		timeout = time.Second / 2
		bufferSize = 1024

		// zero out the values that are assigned in the JustBeforeEach
		prefixedMessage = nil
		batcher = nil
		constructorErr = nil
	})

	JustBeforeEach(func() {
		prefixedMessage = prefixWithLength(messageBytes)
		batcher, constructorErr = egress.NewWriter(byteWriter, droppedCounter, bufferSize, timeout)
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
			bytesWritten, err := batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())
			Expect(bytesWritten).To(Equal(len(messageBytes)))
			Eventually(byteWriter.WriteInput.Message).Should(Receive(Equal(prefixedMessage)))
		})

		It("sends a sentMessages metric", func() {
			_, err := batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())

			Eventually(byteWriter.WriteInput.Message).Should(Receive(Equal(prefixedMessage)))
			Eventually(mockBatcher.BatchAddCounterInput).Should(BeCalled(
				With("DopplerForwarder.sentMessages"),
			))
		})

		It("passes through any chainers", func() {
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
				byteWriter.WriteOutput.Err = make(chan error, 1)
				byteWriter.WriteOutput.Err <- errors.New("boom")
			})

			JustBeforeEach(func() {
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
				// close enough
				infinity := 10
				byteWriter.WriteOutput.Err = make(chan error, infinity)
				for i := 0; i < infinity; i++ {
					byteWriter.WriteOutput.Err <- errors.New("To INFINITY (but not beyond)")
				}
			})

			It("increments the dropped message counter", func() {
				batcher.Write(messageBytes)

				Eventually(droppedCounter.DropInput.Count).Should(Receive(BeNumerically("==", 1)))
				Consistently(func() uint64 { return sender.GetCounter("DopplerForwarder.sentMessages") }).Should(BeEquivalentTo(0))
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

		It("buffers messages while flushing", func() {
			done := make(chan struct{})
			go func() {
				defer close(done)
				_, err := batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
				_, err = batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
				Eventually(byteWriter.WriteCalled).Should(BeCalled())

				By("Writing to the writer even when the writer is already in use")
				_, err = batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
				_, err = batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
				Eventually(byteWriter.WriteCalled).Should(BeCalled())
			}()
			Eventually(done).Should(BeClosed())
		})

		It("flushes multiple messages within buffer when exceeding capacity", func() {
			_, err := batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())

			_, err = batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())
			Eventually(byteWriter.WriteInput.Message).Should(Receive(Equal(twoMessageOutput)))
			Consistently(byteWriter.WriteInput.Message).ShouldNot(Receive())
		})

		It("flushes multiple chainers when exceeding capacity", func() {
			chainers := []metricbatcher.BatchCounterChainer{
				newMockBatchCounterChainer(),
				newMockBatchCounterChainer(),
			}
			_, err := batcher.Write(messageBytes, chainers[0])
			Expect(err).ToNot(HaveOccurred())
			_, err = batcher.Write(messageBytes, chainers[1])
			Expect(err).ToNot(HaveOccurred())

			Eventually(byteWriter.WriteInput).Should(BeCalled(
				With(twoMessageOutput, chainers),
			))
		})

		It("resets the buffer when buffer is flushed", func() {
			_, err := batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())

			_, err = batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())
			Eventually(byteWriter.WriteInput.Message).Should(Receive(Equal(twoMessageOutput)))

			_, err = batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())
			Consistently(byteWriter.WriteInput.Message).Should(HaveLen(0))
		})

		Context("the weighted writer errors once", func() {
			BeforeEach(func() {
				byteWriter.WriteOutput.Err = make(chan error, 10)
				byteWriter.WriteOutput.Err <- errors.New("boom")
				close(byteWriter.WriteOutput.Err)
			})

			It("retries", func() {
				written, err := batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
				Expect(written).To(Equal(len(messageBytes)))

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
				byteWriter.WriteOutput.Err = make(chan error, 100)
				// Close enough.
				infinity := 50
				for i := 0; i < infinity; i++ {
					byteWriter.WriteOutput.Err <- errors.New("To INFINITY (but not beyond)")
				}
			})

			It("uses the dropped message counter to track dropped messages", func() {
				_, err := batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
				_, err = batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())

				Eventually(droppedCounter.DropInput).Should(BeCalled(With(uint32(2))))

				// The buffer should have been reset, so the next write will save
				// to the buffer.
				_, err = batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
				Consistently(droppedCounter.DropInput.Count).ShouldNot(BeCalled())
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
			_, err := batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())

			Eventually(byteWriter.WriteInput.Message).Should(Receive(Equal(prefixedMessage)))

			_, err = batcher.Write(messageBytes)
			Expect(err).ToNot(HaveOccurred())

			Eventually(byteWriter.WriteInput.Message).Should(Receive(Equal(prefixedMessage)))

		})

		It("doesn't reset the timer every write during consistent low load", func() {
			// this might be a bit confusing but we need to make sure that by the
			// end of this test the goroutine has finished running so it doesn't
			// continue to do writes to the next tests's batcher

			ticker := time.NewTicker(75 * time.Millisecond)
			packItUp := make(chan struct{})
			var wg sync.WaitGroup
			wg.Add(1)

			defer func() {
				ticker.Stop()
				close(packItUp)
				wg.Wait()
			}()

			// this prevents changing of the batcher pointer by other tests
			batcher := batcher

			go func() {
				defer wg.Done()
				for {
					select {
					case <-ticker.C:
						batcher.Write([]byte("a"))
					case <-packItUp:
						return
					}
				}
			}()

			Eventually(byteWriter.WriteInput.Message).Should(Receive())
		})

		It("sends a sentMessages metric on flush", func() {

			By("loading batcher up with some messages")
			for i := 0; i < 3; i++ {
				_, err := batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
			}

			By("not blocking during writes")
			Eventually(byteWriter.WriteInput.Message).Should(Receive())
			Eventually(mockBatcher.BatchAddCounterInput).Should(BeCalled(
				With("DopplerForwarder.sentMessages", uint64(3)),
			))
		})

		It("sends a protocol sentMessageCount metric on flush", func() {
			for i := 0; i < 3; i++ {
				_, err := batcher.Write(messageBytes)
				Expect(err).ToNot(HaveOccurred())
			}
			Consistently(mockBatcher.BatchAddCounterInput).ShouldNot(BeCalled(
				With("grpc.sentMessageCount"),
			))

			Eventually(byteWriter.WriteInput.Message).Should(Receive())
			Eventually(mockBatcher.BatchAddCounterInput).Should(BeCalled(
				With("grpc.sentMessageCount", uint64(3)),
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
