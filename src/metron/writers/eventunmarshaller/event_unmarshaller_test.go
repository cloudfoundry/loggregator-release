package eventunmarshaller_test

import (
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	"metron/writers/eventunmarshaller"

	"metron/writers/mocks"

	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"time"
)

var _ = Describe("EventUnmarshaller", func() {
	var (
		mockWriter   *mocks.MockEnvelopeWriter
		unmarshaller *eventunmarshaller.EventUnmarshaller
		event        *events.Envelope
		message      []byte
		fakeSender   *fake.FakeMetricSender
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

		fakeSender = fake.NewFakeMetricSender()
		metricBatcher := metricbatcher.New(fakeSender, time.Millisecond)
		metrics.Initialize(fakeSender, metricBatcher)
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
			Eventually(func() uint64 { return fakeSender.GetCounter("dropsondeUnmarshaller.unmarshalErrors") }).Should(BeEquivalentTo(1))
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
			Eventually(func() uint64 { return fakeSender.GetCounter("dropsondeUnmarshaller.unknownEventTypeReceived") }).Should(BeEquivalentTo(1))
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
			Eventually(func() uint64 { return fakeSender.GetCounter("dropsondeUnmarshaller.logMessageTotal") }).Should(BeEquivalentTo(1))
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
				Eventually(func() uint64 { return fakeSender.GetCounter("dropsondeUnmarshaller.containerMetricReceived") }).Should(BeEquivalentTo(1))

			})

			It("HTTP start event", func() {
				uuid, _ := uuid.NewV4()
				request, _ := http.NewRequest("GET", "https://example.com", nil)
				httpStart := &events.Envelope{
					Origin:    proto.String("origin"),
					EventType: events.Envelope_HttpStart.Enum(),
					HttpStart: factories.NewHttpStart(request, events.PeerType_Client, uuid),
				}
				message, err := proto.Marshal(httpStart)
				Expect(err).ToNot(HaveOccurred())
				_, err = unmarshaller.UnmarshallMessage(message)
				Expect(err).ToNot(HaveOccurred())
				Eventually(func() uint64 { return fakeSender.GetCounter("dropsondeUnmarshaller.httpStartReceived") }).Should(BeEquivalentTo(1))
			})

			It("HTTP stop event", func() {
				uuid, _ := uuid.NewV4()
				request, _ := http.NewRequest("GET", "https://example.com", nil)
				httpStop := &events.Envelope{
					Origin:    proto.String("origin"),
					EventType: events.Envelope_HttpStop.Enum(),
					HttpStop:  factories.NewHttpStop(request, 200, 567, events.PeerType_Client, uuid),
				}
				message, err := proto.Marshal(httpStop)
				Expect(err).ToNot(HaveOccurred())
				_, err = unmarshaller.UnmarshallMessage(message)
				Expect(err).ToNot(HaveOccurred())
				Eventually(func() uint64 { return fakeSender.GetCounter("dropsondeUnmarshaller.httpStopReceived") }).Should(BeEquivalentTo(1))
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
				Eventually(func() uint64 { return fakeSender.GetCounter("dropsondeUnmarshaller.counterEventReceived") }).Should(BeEquivalentTo(1))
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
				Eventually(func() uint64 { return fakeSender.GetCounter("dropsondeUnmarshaller.valueMetricReceived") }).Should(BeEquivalentTo(1))
			})
		})
	})
})
