package unmarshallingeventwriter_test

import (
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation/testhelpers"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	"metron/unmarshallingeventwriter"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"metron/envelopewriter"
)

var _ = Describe("UnmarshallingEventWriter", func() {
	var (
		mockWriter   *envelopewriter.MockEnvelopeWriter
		unmarshaller *unmarshallingeventwriter.UnmarshallingEventWriter
		event        *events.Envelope
		message      []byte
	)

	BeforeEach(func() {
		mockWriter = &envelopewriter.MockEnvelopeWriter{}
		unmarshaller = unmarshallingeventwriter.NewUnmarshallingEventWriter(loggertesthelper.Logger(), mockWriter)
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
	})

	Context("Write", func() {
		It("unmarshalls byte arrays and writes to an EnvelopeWriter", func() {
			bytesWritten, err := unmarshaller.Write(message)
			Expect(err).ToNot(HaveOccurred())
			Expect(bytesWritten).To(Equal(len(message)))

			Expect(mockWriter.Events).To(HaveLen(1))
			Expect(mockWriter.Events[0]).To(Equal(event))
		})

		It("returns an error when it can't unmarshal", func() {
			message = []byte("Bad Message")
			bytesWritten, err := unmarshaller.Write(message)
			Expect(err).To(HaveOccurred())
			Expect(bytesWritten).To(Equal(0))

			Expect(mockWriter.Events).To(HaveLen(0))
		})
	})

	Context("metrics", func() {
		BeforeEach(func() {
			unmarshaller = unmarshallingeventwriter.NewUnmarshallingEventWriter(loggertesthelper.Logger(), mockWriter)
		})

		It("emits the correct metrics context", func() {
			Expect(unmarshaller.Emit().Name).To(Equal("UnmarshallingEventWriter"))
		})

		It("emits a value metric counter", func() {
			unmarshaller.Write(message)
			testhelpers.EventuallyExpectMetric(unmarshaller, "valueMetricReceived", 1)

			Eventually(fakeEventEmitter.GetMessages).Should(HaveLen(1))
			Expect(fakeEventEmitter.GetMessages()[0].Event.(*events.CounterEvent)).To(Equal(&events.CounterEvent{
				Name:  proto.String("UnmarshallingEventWriter.valueMetricReceived"),
				Delta: proto.Uint64(1),
			}))
		})

		It("emits a total log message counter", func() {
			envelope1 := &events.Envelope{
				Origin:     proto.String("fake-origin-3"),
				EventType:  events.Envelope_LogMessage.Enum(),
				LogMessage: factories.NewLogMessage(events.LogMessage_OUT, "test log message 1", "fake-app-id-1", "DEA"),
			}

			envelope2 := &events.Envelope{
				Origin:     proto.String("fake-origin-3"),
				EventType:  events.Envelope_LogMessage.Enum(),
				LogMessage: factories.NewLogMessage(events.LogMessage_OUT, "test log message 2", "fake-app-id-2", "DEA"),
			}

			message1, _ := proto.Marshal(envelope1)
			message2, _ := proto.Marshal(envelope2)

			unmarshaller.Write(message1)
			unmarshaller.Write(message1)
			unmarshaller.Write(message2)

			Eventually(func() uint64 {
				return getTotalLogMessageCount(unmarshaller)
			}).Should(BeNumerically("==", 3))

			Eventually(fakeEventEmitter.GetMessages).Should(HaveLen(1))
			Expect(fakeEventEmitter.GetMessages()[0].Event.(*events.CounterEvent)).To(Equal(&events.CounterEvent{
				Name:  proto.String("UnmarshallingEventWriter.logMessageTotal"),
				Delta: proto.Uint64(3),
			}))
		})

		It("emits an unmarshal error counter", func() {
			unmarshaller.Write([]byte{1, 2, 3})
			testhelpers.EventuallyExpectMetric(unmarshaller, "unmarshalErrors", 1)

			Eventually(fakeEventEmitter.GetMessages).Should(HaveLen(1))
			Expect(fakeEventEmitter.GetMessages()[0].Event.(*events.CounterEvent)).To(Equal(&events.CounterEvent{
				Name:  proto.String("UnmarshallingEventWriter.unmarshalErrors"),
				Delta: proto.Uint64(1),
			}))
		})

		It("counts unknown message types", func() {
			unexpectedMessageType := events.Envelope_EventType(1)
			envelope1 := &events.Envelope{
				Origin:     proto.String("fake-origin-3"),
				EventType:  &unexpectedMessageType,
				LogMessage: factories.NewLogMessage(events.LogMessage_OUT, "test log message 1", "fake-app-id-1", "DEA"),
			}
			message1, err := proto.Marshal(envelope1)
			Expect(err).NotTo(HaveOccurred())

			unmarshaller.Write(message1)

			Eventually(fakeEventEmitter.GetMessages).Should(HaveLen(1))
			Expect(fakeEventEmitter.GetMessages()[0].Event.(*events.CounterEvent)).To(Equal(&events.CounterEvent{
				Name:  proto.String("UnmarshallingEventWriter.unknownEventTypeReceived"),
				Delta: proto.Uint64(1),
			}))

		})
	})
})

func getTotalLogMessageCount(instrumentable instrumentation.Instrumentable) uint64 {
	for _, metric := range instrumentable.Emit().Metrics {
		if metric.Name == "logMessageTotal" {
			return metric.Value.(uint64)
		}
	}
	return uint64(0)
}
