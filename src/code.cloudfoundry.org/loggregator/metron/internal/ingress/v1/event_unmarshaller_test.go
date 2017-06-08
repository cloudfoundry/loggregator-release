package v1_test

import (
	ingress "code.cloudfoundry.org/loggregator/metron/internal/ingress/v1"

	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	. "github.com/apoydence/eachers"
	"github.com/apoydence/eachers/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EventUnmarshaller", func() {
	var (
		mockWriter   *MockEnvelopeWriter
		unmarshaller *ingress.EventUnmarshaller
		event        *events.Envelope
		message      []byte
		mockBatcher  *mockEventBatcher
		mockChainer  *mockBatchCounterChainer
	)

	BeforeEach(func() {
		mockWriter = &MockEnvelopeWriter{}
		mockBatcher = newMockEventBatcher()
		mockChainer = newMockBatchCounterChainer()
		testhelpers.AlwaysReturn(mockBatcher.BatchCounterOutput, mockChainer)
		testhelpers.AlwaysReturn(mockChainer.SetTagOutput, mockChainer)

		unmarshaller = ingress.NewUnMarshaller(mockWriter, mockBatcher)
		event = &events.Envelope{
			Origin:      proto.String("fake-origin-3"),
			EventType:   events.Envelope_ValueMetric.Enum(),
			ValueMetric: factories.NewValueMetric("value-name", 1.0, "units"),
		}
		message, _ = proto.Marshal(event)

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

	Context("metrics", func() {
		It("emits unmarshal errors", func() {
			_, err := unmarshaller.UnmarshallMessage([]byte("illegal envelope"))
			Expect(err).To(HaveOccurred())
			Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
				With("dropsondeUnmarshaller.unmarshalErrors"),
			))
		})

		It("emits envelope counters tagged with event type and protocol", func() {
			logMessage := &events.Envelope{
				Origin:    proto.String("origin"),
				EventType: events.Envelope_LogMessage.Enum(),
				LogMessage: &events.LogMessage{
					Message:     []byte("some log message"),
					MessageType: events.LogMessage_OUT.Enum(),
					Timestamp:   proto.Int64(1234),
				},
			}
			message, err := proto.Marshal(logMessage)
			Expect(err).ToNot(HaveOccurred())
			_, err = unmarshaller.UnmarshallMessage(message)
			Expect(err).ToNot(HaveOccurred())
			Eventually(mockBatcher.BatchCounterInput).Should(BeCalled(
				With("dropsondeUnmarshaller.receivedEnvelopes"),
			))
			Eventually(mockChainer.SetTagInput).Should(BeCalled(
				With("protocol", "udp"),
				With("event_type", "LogMessage"),
			))
			Eventually(mockChainer.IncrementCalled).Should(BeCalled())
		})

		It("emits unknown event types", func() {
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

			_, err = unmarshaller.UnmarshallMessage(message)
			Expect(err).To(HaveOccurred())
			Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
				With("dropsondeUnmarshaller.unknownEventTypeReceived"),
			))
		})

		It("emits logMessageTotal", func() {
			logMessage := &events.Envelope{
				Origin:    proto.String("origin"),
				EventType: events.Envelope_LogMessage.Enum(),
				LogMessage: &events.LogMessage{
					Message:     []byte("some log message"),
					MessageType: events.LogMessage_OUT.Enum(),
					Timestamp:   proto.Int64(1234),
				},
			}

			message, err := proto.Marshal(logMessage)
			Expect(err).ToNot(HaveOccurred())
			_, err = unmarshaller.UnmarshallMessage(message)
			Expect(err).ToNot(HaveOccurred())
			Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
				With("dropsondeUnmarshaller.logMessageTotal"),
			))
		})

		Describe("emits counters for", func() {
			It("container metrics", func() {
				containerMetric := &events.Envelope{
					Origin:          proto.String("origin"),
					EventType:       events.Envelope_ContainerMetric.Enum(),
					ContainerMetric: factories.NewContainerMetric("appId", 0, 3.0, 23, 23),
				}
				message, err := proto.Marshal(containerMetric)
				Expect(err).ToNot(HaveOccurred())
				_, err = unmarshaller.UnmarshallMessage(message)
				Expect(err).ToNot(HaveOccurred())
				Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
					With("dropsondeUnmarshaller.containerMetricReceived"),
				))
			})

			It("CounterEvent", func() {
				counterEvent := &events.Envelope{
					Origin:       proto.String("origin"),
					EventType:    events.Envelope_CounterEvent.Enum(),
					CounterEvent: factories.NewCounterEvent("counter", 5),
				}
				message, err := proto.Marshal(counterEvent)
				Expect(err).ToNot(HaveOccurred())
				_, err = unmarshaller.UnmarshallMessage(message)
				Expect(err).ToNot(HaveOccurred())
				Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
					With("dropsondeUnmarshaller.counterEventReceived"),
				))
			})

			It("ValueMetric", func() {
				valueMetric := &events.Envelope{
					Origin:      proto.String("origin"),
					EventType:   events.Envelope_ValueMetric.Enum(),
					ValueMetric: factories.NewValueMetric("name", 3.0, "unit"),
				}
				message, err := proto.Marshal(valueMetric)
				Expect(err).ToNot(HaveOccurred())
				_, err = unmarshaller.UnmarshallMessage(message)
				Expect(err).ToNot(HaveOccurred())
				Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
					With("dropsondeUnmarshaller.valueMetricReceived"),
				))
			})
		})

		Context("Validation", func() {
			validateIt := func(name string, val int32) {
				It("emits an unmarshalError for an incomplete "+name, func() {
					incompleteEvent := &events.Envelope{
						Origin:    proto.String("fake-origin-2"),
						EventType: events.Envelope_EventType(val).Enum(),
					}
					message, err := proto.Marshal(incompleteEvent)
					Expect(err).ToNot(HaveOccurred())

					_, err = unmarshaller.UnmarshallMessage(message)
					Expect(err).To(HaveOccurred())

					Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
						With("dropsondeUnmarshaller.unmarshalErrors"),
					))
				})
			}

			for name, val := range events.Envelope_EventType_value {
				validateIt(name, val)
			}
		})
	})
})
