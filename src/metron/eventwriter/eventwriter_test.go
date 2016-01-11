package eventwriter_test

import (
	"metron/eventwriter"
	"metron/writers/mocks"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	"github.com/cloudfoundry/dropsonde/emitter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EventWriter", func() {
	It("writes emitted events", func() {
		writer := mocks.MockEnvelopeWriter{}
		ew := eventwriter.New("Africa", &writer)

		event := &events.ValueMetric{
			Name:  proto.String("ValueName"),
			Value: proto.Float64(13),
			Unit:  proto.String("giraffes"),
		}

		ew.Emit(event)
		Expect(writer.Events).To(HaveLen(1))
		Expect(writer.Events[0].GetOrigin()).To(Equal("Africa"))
		Expect(writer.Events[0].GetEventType()).To(Equal(events.Envelope_ValueMetric))
		Expect(writer.Events[0].GetValueMetric()).To(Equal(event))
	})

	It("satisfies the dropsonde/emitter/EventEmitter interface", func() {
		var ee emitter.EventEmitter
		ee = eventwriter.New("Africa", &mocks.MockEnvelopeWriter{})
		Expect(ee).NotTo(BeNil())
	})
})
