package picker_test

import (
	"metron/writers/picker"
	"time"

	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	"github.com/cloudfoundry/gosteno"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Picker", func() {
	var (
		writerPicker   *picker.Picker
		envWriters     []picker.WeightedByteWriter
		constructorErr error
		sender         *fake.FakeMetricSender
		defaultWriter  picker.WeightedByteWriter
	)

	BeforeEach(func() {
		sender = fake.NewFakeMetricSender()
		metrics.Initialize(sender, metricbatcher.New(sender, time.Millisecond*10))
	})

	JustBeforeEach(func() {
		logger := gosteno.NewLogger("TestLogger")
		writerPicker, constructorErr = picker.New(logger, defaultWriter, envWriters...)
	})

	Describe("NewPicker", func() {
		Context("without any forwarders", func() {
			BeforeEach(func() {
				envWriters = nil
			})
			It("returns an error", func() {
				Expect(constructorErr).To(HaveOccurred())
				Expect(writerPicker).To(BeNil())
			})
		})

		Context("with one or more forwarders", func() {
			BeforeEach(func() {
				envWriters = []picker.WeightedByteWriter{
					newMockByteArrayWriter(),
				}
			})

			It("returns a non-nil picker", func() {
				Expect(constructorErr).ToNot(HaveOccurred())
				Expect(writerPicker).ToNot(BeNil())
			})
		})
	})

	Describe("Write", func() {
		var envelope *events.Envelope

		BeforeEach(func() {
			envelope = &events.Envelope{
				Origin:    proto.String("The Negative Zone"),
				EventType: events.Envelope_LogMessage.Enum(),
			}
		})

		Context("with an invalid envelope", func() {
			BeforeEach(func() {
				envelope = &events.Envelope{}
				mockWriter := newMockByteArrayWriter()
				mockWriter.WeightOutput.ret0 <- 1
				envWriters = []picker.WeightedByteWriter{
					mockWriter,
				}
			})

			It("counts marshal errors", func() {
				writerPicker.Write(envelope)
				Eventually(func() uint64 { return sender.GetCounter("dropsondeMarshaller.marshalErrors") }).Should(BeEquivalentTo(1))
			})
		})

		Context("with one writer", func() {
			var writer *mockByteArrayWriter

			BeforeEach(func() {
				writer = newMockByteArrayWriter()
				writer.WeightOutput.ret0 <- 1
				envWriters = []picker.WeightedByteWriter{
					writer,
				}
			})

			It("writes messages to the writer", func() {
				writerPicker.Write(envelope)
				expected, err := proto.Marshal(envelope)
				Expect(err).ToNot(HaveOccurred())
				Eventually(writer.WriteInput.message).Should(Receive(Equal(expected)))
				Consistently(func() uint64 { return sender.GetCounter("dropsondeMarshaller.marshalErrors") }).Should(BeEquivalentTo(0))
			})
		})

		Context("with multiple writers", func() {
			var firstMockWriter, secondMockWriter *mockByteArrayWriter

			BeforeEach(func() {
				firstMockWriter = newMockByteArrayWriter()
				secondMockWriter = newMockByteArrayWriter()
				envWriters = []picker.WeightedByteWriter{
					firstMockWriter,
					secondMockWriter,
				}
			})

			It("only writes each message once", func(done Done) {
				defer close(done)
				firstMockWriter.WeightOutput.ret0 <- 1
				secondMockWriter.WeightOutput.ret0 <- 1
				writerPicker.Write(&events.Envelope{})
				select {
				case <-firstMockWriter.WriteCalled:
				case <-secondMockWriter.WriteCalled:
				}

				Consistently(firstMockWriter.WriteCalled).ShouldNot(Receive())
				Consistently(secondMockWriter.WriteCalled).ShouldNot(Receive())
			})

			It("writes messages to both writers", func() {
				for i := 0; i < 100; i++ {
					firstMockWriter.WeightOutput.ret0 <- 1
					secondMockWriter.WeightOutput.ret0 <- 1
					writerPicker.Write(&events.Envelope{})
				}

				Expect(len(firstMockWriter.WriteCalled)).To(BeNumerically("~", 50, 10))
				Expect(len(secondMockWriter.WriteCalled)).To(BeNumerically("~", 50, 10))
			})

			It("doesn't write to writer with zero weight", func() {
				for i := 0; i < 100; i++ {
					firstMockWriter.WeightOutput.ret0 <- 0
					secondMockWriter.WeightOutput.ret0 <- 2

					writerPicker.Write(&events.Envelope{})
				}

				Expect(len(firstMockWriter.WriteCalled)).To(Equal(0))
				Expect(len(secondMockWriter.WriteCalled)).To(Equal(100))
			})

			It("writes messages to both writers with a weighted distribution", func() {
				for i := 0; i < 100; i++ {
					firstMockWriter.WeightOutput.ret0 <- 3
					secondMockWriter.WeightOutput.ret0 <- 7
					writerPicker.Write(&events.Envelope{})
				}

				Expect(len(firstMockWriter.WriteCalled)).To(BeNumerically("~", 30, 10))
				Expect(len(secondMockWriter.WriteCalled)).To(BeNumerically("~", 70, 10))
			})

			It("writes messages to only one writer", func() {
				for i := 0; i < 100; i++ {
					firstMockWriter.WeightOutput.ret0 <- 0
					secondMockWriter.WeightOutput.ret0 <- 1

					writerPicker.Write(&events.Envelope{})
				}

				Expect(len(firstMockWriter.WriteCalled)).To(Equal(0))
				Expect(len(secondMockWriter.WriteCalled)).To(Equal(100))
			})

			It("writes messages to both writers with a weighted distribution", func() {
				for i := 0; i < 100; i++ {
					firstMockWriter.WeightOutput.ret0 <- 9
					secondMockWriter.WeightOutput.ret0 <- 1
					writerPicker.Write(&events.Envelope{})
				}

				Expect(len(firstMockWriter.WriteCalled)).To(BeNumerically("~", 90, 10))
				Expect(len(secondMockWriter.WriteCalled)).To(BeNumerically("~", 10, 10))
			})

			Context("with default writer", func() {
				BeforeEach(func() {
					defaultWriter = firstMockWriter
				})

				It("picks the default writer if all weights are zero", func() {
					firstMockWriter.WeightOutput.ret0 <- 0
					secondMockWriter.WeightOutput.ret0 <- 0
					writerPicker.Write(&events.Envelope{})

					Expect(len(firstMockWriter.WriteCalled)).To(Equal(1))
					Expect(len(secondMockWriter.WriteCalled)).To(Equal(0))
				})
			})

		})

	})
})
