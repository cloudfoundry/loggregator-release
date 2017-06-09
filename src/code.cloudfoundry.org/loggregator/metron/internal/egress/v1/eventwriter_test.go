package v1_test

import (
	egress "code.cloudfoundry.org/loggregator/metron/internal/egress/v1"

	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EventWriter", func() {
	var (
		mockWriter  *MockEnvelopeWriter
		eventWriter *egress.EventWriter
	)

	BeforeEach(func() {
		mockWriter = &MockEnvelopeWriter{}
		eventWriter = egress.New("Africa")
	})

	Describe("Emit", func() {
		It("writes emitted events", func() {
			eventWriter.SetWriter(mockWriter)

			event := &events.ValueMetric{
				Name:  proto.String("ValueName"),
				Value: proto.Float64(13),
				Unit:  proto.String("giraffes"),
			}
			eventWriter.Emit(event)

			Expect(mockWriter.Events).To(HaveLen(1))
			Expect(mockWriter.Events[0].GetOrigin()).To(Equal("Africa"))
			Expect(mockWriter.Events[0].GetEventType()).To(Equal(events.Envelope_ValueMetric))
			Expect(mockWriter.Events[0].GetValueMetric()).To(Equal(event))
		})

		It("returns an error with a sane message when emitting without a writer", func() {
			event := &events.ValueMetric{
				Name:  proto.String("ValueName"),
				Value: proto.Float64(13),
				Unit:  proto.String("giraffes"),
			}
			err := eventWriter.Emit(event)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("EventWriter: No envelope writer set (see SetWriter)"))
		})
	})

	Describe("EmitEnvelope", func() {
		It("writes emitted events", func() {
			eventWriter.SetWriter(mockWriter)

			event := &events.Envelope{
				Origin:    proto.String("foo"),
				EventType: events.Envelope_ValueMetric.Enum(),
				ValueMetric: &events.ValueMetric{
					Name:  proto.String("ValueName"),
					Value: proto.Float64(13),
					Unit:  proto.String("giraffes"),
				},
			}
			eventWriter.EmitEnvelope(event)

			Expect(mockWriter.Events).To(HaveLen(1))
			Expect(mockWriter.Events[0]).To(Equal(event))
		})

		It("returns an error with a sane message when emitting without a writer", func() {
			event := &events.Envelope{
				Origin:    proto.String("foo"),
				EventType: events.Envelope_ValueMetric.Enum(),
				ValueMetric: &events.ValueMetric{
					Name:  proto.String("ValueName"),
					Value: proto.Float64(13),
					Unit:  proto.String("giraffes"),
				},
			}
			err := eventWriter.EmitEnvelope(event)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("EventWriter: No envelope writer set (see SetWriter)"))
		})
	})
})

// Make sure eventwriter satisfies "github.com/cloudfoundry/dropsonde".EventEmitter
var _ dropsonde.EventEmitter = egress.New("Africa")
