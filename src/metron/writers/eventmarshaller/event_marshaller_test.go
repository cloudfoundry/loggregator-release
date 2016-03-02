package eventmarshaller_test

import (
	"metron/writers/eventmarshaller"
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

var _ = Describe("EventMarhsaller", func() {
	var (
		marshaller *eventmarshaller.EventMarshaller
		sender     *fake.FakeMetricSender
		writer     *mockByteWriter
	)

	BeforeEach(func() {
		sender = fake.NewFakeMetricSender()
		metrics.Initialize(sender, metricbatcher.New(sender, time.Millisecond*10))
	})

	JustBeforeEach(func() {
		logger := gosteno.NewLogger("TestLogger")
		marshaller = eventmarshaller.New(logger, writer)
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
				writer = newMockByteWriter()
				envelope = &events.Envelope{}
				close(writer.WriteOutput.sentLength)
				close(writer.WriteOutput.err)
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
				writer = newMockByteWriter()
				close(writer.WriteOutput.sentLength)
				close(writer.WriteOutput.err)
			})

			It("writes messages to the writer", func() {
				marshaller.Write(envelope)
				expected, err := proto.Marshal(envelope)
				Expect(err).ToNot(HaveOccurred())
				Eventually(writer.WriteInput.message).Should(Receive(Equal(expected)))
				Consistently(func() uint64 { return sender.GetCounter("dropsondeMarshaller.marshalErrors") }).Should(BeEquivalentTo(0))
			})
		})
	})
})
