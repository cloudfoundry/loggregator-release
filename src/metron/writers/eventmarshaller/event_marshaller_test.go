package eventmarshaller_test

import (
	"metron/writers/eventmarshaller"
	"time"

	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

var _ = Describe("EventMarhsaller", func() {
	var (
		marshaller *eventmarshaller.EventMarshaller
		sender     *fake.FakeMetricSender
		writer     *mockByteWriter
	)

	BeforeEach(func() {
		writer = newMockByteWriter()
		sender = fake.NewFakeMetricSender()
		metrics.Initialize(sender, metricbatcher.New(sender, time.Millisecond*10))
	})

	JustBeforeEach(func() {
		logger := gosteno.NewLogger("TestLogger")
		marshaller = eventmarshaller.New(logger)
		marshaller.SetWriter(writer)
	})

	Describe("Write", func() {
		var envelope *events.Envelope

		Context("with a nil writer", func() {
			BeforeEach(func() {
				envelope = &events.Envelope{
					Origin:    proto.String("The Negative Zone"),
					EventType: events.Envelope_LogMessage.Enum(),
				}
			})

			JustBeforeEach(func() {
				marshaller.SetWriter(nil)
			})

			It("does not panic", func() {
				Expect(func() {
					marshaller.Write(envelope)
				}).ToNot(Panic())
			})

			It("counts messages written while byteWriter was nil", func() {
				marshaller.Write(envelope)
				Eventually(func() uint64 { return sender.GetCounter("dropsondeMarshaller.nilByteWriterWrites") }).Should(BeEquivalentTo(1))
			})
		})

		Context("with an invalid envelope", func() {
			BeforeEach(func() {
				envelope = &events.Envelope{}
				close(writer.WriteOutput.SentLength)
				close(writer.WriteOutput.Err)
			})

			It("counts marshal errors", func() {
				marshaller.Write(envelope)
				Eventually(func() uint64 { return sender.GetCounter("dropsondeMarshaller.marshalErrors") }).Should(BeEquivalentTo(1))
			})

			It("doesn't write the bytes", func() {
				marshaller.Write(envelope)
				Consistently(writer.WriteCalled).ShouldNot(Receive())
			})
		})

		Context("with writer", func() {
			BeforeEach(func() {
				close(writer.WriteOutput.SentLength)
				close(writer.WriteOutput.Err)
				envelope = &events.Envelope{
					Origin:    proto.String("The Negative Zone"),
					EventType: events.Envelope_LogMessage.Enum(),
				}
			})

			It("writes messages to the writer", func() {
				marshaller.Write(envelope)
				expected, err := proto.Marshal(envelope)
				Expect(err).ToNot(HaveOccurred())
				Eventually(writer.WriteInput).Should(BeCalled(With(expected)))
				Consistently(func() uint64 { return sender.GetCounter("dropsondeMarshaller.marshalErrors") }).Should(BeEquivalentTo(0))
			})
		})
	})

	Describe("SetWriter", func() {
		It("writes to the new writer", func() {
			newWriter := newMockByteWriter()
			close(newWriter.WriteOutput.Err)
			close(newWriter.WriteOutput.SentLength)
			marshaller.SetWriter(newWriter)

			envelope := &events.Envelope{
				Origin:    proto.String("The Negative Zone"),
				EventType: events.Envelope_LogMessage.Enum(),
			}
			marshaller.Write(envelope)

			expected, err := proto.Marshal(envelope)
			Expect(err).ToNot(HaveOccurred())
			Consistently(writer.WriteInput).ShouldNot(BeCalled())
			Eventually(newWriter.WriteInput).Should(BeCalled(With(expected)))
		})
	})
})
