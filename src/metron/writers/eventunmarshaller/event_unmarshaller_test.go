package eventunmarshaller_test

import (
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	"metron/writers/eventunmarshaller"

	"metron/writers/mocks"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EventUnmarshaller", func() {
	var (
		mockWriter   *mocks.MockEnvelopeWriter
		unmarshaller *eventunmarshaller.EventUnmarshaller
		event        *events.Envelope
		message      []byte
	)

	BeforeEach(func() {
		mockWriter = &mocks.MockEnvelopeWriter{}
		unmarshaller = eventunmarshaller.New(mockWriter, loggertesthelper.Logger())
		event = &events.Envelope{
			Origin:      proto.String("fake-origin-3"),
			EventType:   events.Envelope_ValueMetric.Enum(),
			ValueMetric: factories.NewValueMetric("value-name", 1.0, "units"),
		}
		message, _ = proto.Marshal(event)

		fakeEventEmitter.Reset()
		metricBatcher.Reset()
	})

	Context("UnmarshallMessage", func() {
		It("unmarshalls bytes", func() {
			output, _ := unmarshaller.UnmarshallMessage(message)

			Expect(output).To(Equal(event))
		})

		It("handles bad input gracefully", func() {
			output, err := unmarshaller.UnmarshallMessage(make([]byte, 4))
			Expect(output).To(BeNil())
			Expect(err).To(HaveOccurred())
		})

		It("doesn't write unknown event types", func() {
			unknownEventTypeMessage := &events.Envelope{
				Origin:    proto.String("fake-origin-2"),
				EventType: events.Envelope_EventType(2000).Enum(),
				ValueMetric: &events.ValueMetric{
					Name:  proto.String("fake-metric-name"),
					Value: proto.Float64(42),
					Unit:  proto.String("fake-unit"),
				},
			}
			message, err := proto.Marshal(unknownEventTypeMessage)
			Expect(err).ToNot(HaveOccurred())

			output, err := unmarshaller.UnmarshallMessage(message)
			Expect(output).To(BeNil())
			Expect(err).To(HaveOccurred())
		})
	})

	Context("Write", func() {
		It("unmarshalls byte arrays and writes to an EnvelopeWriter", func() {
			unmarshaller.Write(message)

			Expect(mockWriter.Events).To(HaveLen(1))
			Expect(mockWriter.Events[0]).To(Equal(event))
		})

		It("returns an error when it can't unmarshal", func() {
			message = []byte("Bad Message")
			unmarshaller.Write(message)

			Expect(mockWriter.Events).To(HaveLen(0))
		})
	})
})
