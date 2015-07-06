package messageaggregator_test

import (
	"fmt"
	"metron/writers/messageaggregator"
	"time"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	"metron/writers/mocks"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MessageAggregator", func() {
	var (
		mockWriter        *mocks.MockEnvelopeWriter
		messageAggregator *messageaggregator.MessageAggregator
		originalTTL       time.Duration
	)

	BeforeEach(func() {
		mockWriter = &mocks.MockEnvelopeWriter{}
		messageAggregator = messageaggregator.New(mockWriter, loggertesthelper.Logger(), true)
		originalTTL = messageaggregator.MaxTTL
	})

	AfterEach(func() {
		messageaggregator.MaxTTL = originalTTL
	})

	It("passes non-marshallable messages through", func() {
		inputMessage := &events.Envelope{}
		messageAggregator.Write(inputMessage)

		Expect(mockWriter.Events).To(HaveLen(1))
		Expect(mockWriter.Events[0]).To(Equal(inputMessage))
	})

	It("passes value messages through", func() {
		inputMessage := createValueMessage()
		messageAggregator.Write(inputMessage)

		Expect(mockWriter.Events).To(HaveLen(1))
		Expect(mockWriter.Events[0]).To(Equal(inputMessage))
	})

	Describe("counter processing", func() {
		It("sets the Total field on a CounterEvent ", func() {
			messageAggregator.Write(createCounterMessage("total", "fake-origin-4"))

			Expect(mockWriter.Events).To(HaveLen(1))
			outputMessage := mockWriter.Events[0]
			Expect(outputMessage.GetEventType()).To(Equal(events.Envelope_CounterEvent))
			expectCorrectCounterNameDeltaAndTotal(outputMessage, "total", 4, 4)
		})

		It("accumulates Deltas for CounterEvents with the same name and origin", func() {
			messageAggregator.Write(createCounterMessage("total", "fake-origin-4"))
			messageAggregator.Write(createCounterMessage("total", "fake-origin-4"))

			Expect(mockWriter.Events).To(HaveLen(2))
			outputMessage := mockWriter.Events[1]

			expectCorrectCounterNameDeltaAndTotal(outputMessage, "total", 4, 8)
		})

		It("accumulates differently-named counters separately", func() {
			messageAggregator.Write(createCounterMessage("total1", "fake-origin-4"))
			messageAggregator.Write(createCounterMessage("total2", "fake-origin-4"))

			Expect(mockWriter.Events).To(HaveLen(2))
			expectCorrectCounterNameDeltaAndTotal(mockWriter.Events[0], "total1", 4, 4)
			expectCorrectCounterNameDeltaAndTotal(mockWriter.Events[1], "total2", 4, 4)
		})

		It("does not accumulate for counters when receiving a non-counter event", func() {
			messageAggregator.Write(createValueMessage())
			messageAggregator.Write(createCounterMessage("counter1", "fake-origin-4"))

			Expect(mockWriter.Events).To(HaveLen(2))
			Expect(mockWriter.Events[0].GetEventType()).To(Equal(events.Envelope_ValueMetric))
			Expect(mockWriter.Events[1].GetEventType()).To(Equal(events.Envelope_CounterEvent))
			expectCorrectCounterNameDeltaAndTotal(mockWriter.Events[1], "counter1", 4, 4)
		})

		It("accumulates independently for different origins", func() {
			messageAggregator.Write(createCounterMessage("counter1", "fake-origin-4"))
			messageAggregator.Write(createCounterMessage("counter1", "fake-origin-5"))
			messageAggregator.Write(createCounterMessage("counter1", "fake-origin-4"))

			Expect(mockWriter.Events).To(HaveLen(3))

			Expect(mockWriter.Events[0].GetOrigin()).To(Equal("fake-origin-4"))
			expectCorrectCounterNameDeltaAndTotal(mockWriter.Events[0], "counter1", 4, 4)

			Expect(mockWriter.Events[1].GetOrigin()).To(Equal("fake-origin-5"))
			expectCorrectCounterNameDeltaAndTotal(mockWriter.Events[1], "counter1", 4, 4)

			Expect(mockWriter.Events[2].GetOrigin()).To(Equal("fake-origin-4"))
			expectCorrectCounterNameDeltaAndTotal(mockWriter.Events[2], "counter1", 4, 8)
		})
	})

	Context("single StartStop message", func() {
		var outputMessage *events.Envelope
		BeforeEach(func() {
			messageAggregator.Write(createStartMessage(123, events.PeerType_Client))
			messageAggregator.Write(createStopMessage(123, events.PeerType_Client))

			outputMessage = mockWriter.Events[0]
		})

		It("populates all fields in the StartStop message correctly", func() {
			Expect(outputMessage.GetHttpStartStop()).To(Equal(createStartStopMessage(123, events.PeerType_Client).GetHttpStartStop()))

		})

		It("populates all fields in the Envelope correctly", func() {
			Expect(outputMessage.GetOrigin()).To(Equal("fake-origin-2"))
			Expect(outputMessage.GetTimestamp()).ToNot(BeZero())
			Expect(outputMessage.GetEventType()).To(Equal(events.Envelope_HttpStartStop))
		})
	})

	It("does not send a combined event if there only is a stop event", func() {
		messageAggregator.Write(createStopMessage(123, events.PeerType_Client))
		Consistently(func() int { return len(mockWriter.Events) }).Should(Equal(0))
	})

	Context("message expiry", func() {
		BeforeEach(func() {
			messageaggregator.MaxTTL = 0
		})

		It("does not send a combined event if the stop event doesn't arrive within the TTL", func() {
			messageAggregator.Write(createStartMessage(123, events.PeerType_Client))
			time.Sleep(1)
			messageAggregator.Write(createStopMessage(123, events.PeerType_Client))

			Consistently(func() int { return len(mockWriter.Events) }).Should(Equal(0))
		})
	})

	var metricValue = func(name string) interface{} {
		for _, metric := range messageAggregator.Emit().Metrics {
			if metric.Name == name {
				return metric.Value
			}
		}
		return nil
	}

	Context("metrics", func() {
		var eventuallyExpectMetric = func(name string, value uint64) {
			Eventually(func() interface{} {
				return metricValue(name)
			}).Should(Equal(value), fmt.Sprintf("Metric %s was incorrect", name))
		}

		It("emits a HTTP start counter", func() {
			messageAggregator.Write(createStartMessage(123, events.PeerType_Client))
			eventuallyExpectMetric("httpStartReceived", 1)
		})

		It("emits a HTTP stop counter", func() {
			messageAggregator.Write(createStopMessage(123, events.PeerType_Client))
			eventuallyExpectMetric("httpStopReceived", 1)
		})

		It("emits a HTTP StartStop counter", func() {
			messageAggregator.Write(createStartMessage(123, events.PeerType_Client))
			messageAggregator.Write(createStopMessage(123, events.PeerType_Client))
			eventuallyExpectMetric("httpStartStopEmitted", 1)
		})

		It("emits a counter for uncategorized events", func() {
			messageAggregator.Write(createValueMessage())
			eventuallyExpectMetric("uncategorizedEvents", 1)
		})

		It("emits a counter for unmatched start events", func() {
			messageaggregator.MaxTTL = 0
			messageAggregator.Write(createStartMessage(123, events.PeerType_Client))
			time.Sleep(1)
			messageAggregator.Write(createStartMessage(123, events.PeerType_Client))
			eventuallyExpectMetric("httpUnmatchedStartReceived", 1)
		})

		It("emits a counter for unmatched stop events", func() {
			messageAggregator.Write(createStopMessage(123, events.PeerType_Client))
			eventuallyExpectMetric("httpUnmatchedStopReceived", 1)
		})

		It("emits a counter for counter events", func() {
			messageAggregator.Write(createCounterMessage("counter1", "fake-origin-1"))
			eventuallyExpectMetric("counterEventReceived", 1)

			// since we're counting counters, let's make sure we're not adding their deltas
			messageAggregator.Write(createCounterMessage("counter1", "fake-origin-1"))
			eventuallyExpectMetric("counterEventReceived", 2)
		})
	})
})

func createStartMessage(requestId uint64, peerType events.PeerType) *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String("fake-origin-1"),
		EventType: events.Envelope_HttpStart.Enum(),
		HttpStart: &events.HttpStart{
			Timestamp: proto.Int64(1),
			RequestId: &events.UUID{
				Low:  proto.Uint64(requestId),
				High: proto.Uint64(requestId + 1),
			},
			PeerType:      &peerType,
			Method:        events.Method_GET.Enum(),
			Uri:           proto.String("fake-uri-1"),
			RemoteAddress: proto.String("fake-remote-addr-1"),
			UserAgent:     proto.String("fake-user-agent-1"),
			ParentRequestId: &events.UUID{
				Low:  proto.Uint64(2),
				High: proto.Uint64(3),
			},
			InstanceIndex: proto.Int32(6),
			InstanceId:    proto.String("fake-instance-id-1"),
		},
	}
}

func createStopMessage(requestId uint64, peerType events.PeerType) *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String("fake-origin-2"),
		EventType: events.Envelope_HttpStop.Enum(),
		HttpStop: &events.HttpStop{
			Timestamp: proto.Int64(100),
			Uri:       proto.String("fake-uri-2"),
			RequestId: &events.UUID{
				Low:  proto.Uint64(requestId),
				High: proto.Uint64(requestId + 1),
			},
			PeerType:      &peerType,
			StatusCode:    proto.Int32(103),
			ContentLength: proto.Int64(104),
			ApplicationId: &events.UUID{
				Low:  proto.Uint64(105),
				High: proto.Uint64(106),
			},
		},
	}
}

func createStartStopMessage(requestId uint64, peerType events.PeerType) *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String("fake-origin-2"),
		EventType: events.Envelope_HttpStartStop.Enum(),
		HttpStartStop: &events.HttpStartStop{
			StartTimestamp: proto.Int64(1),
			StopTimestamp:  proto.Int64(100),
			RequestId: &events.UUID{
				Low:  proto.Uint64(requestId),
				High: proto.Uint64(requestId + 1),
			},
			PeerType:      &peerType,
			Method:        events.Method_GET.Enum(),
			Uri:           proto.String("fake-uri-1"),
			RemoteAddress: proto.String("fake-remote-addr-1"),
			UserAgent:     proto.String("fake-user-agent-1"),
			StatusCode:    proto.Int32(103),
			ContentLength: proto.Int64(104),
			ParentRequestId: &events.UUID{
				Low:  proto.Uint64(2),
				High: proto.Uint64(3),
			},
			ApplicationId: &events.UUID{
				Low:  proto.Uint64(105),
				High: proto.Uint64(106),
			},
			InstanceIndex: proto.Int32(6),
			InstanceId:    proto.String("fake-instance-id-1"),
		},
	}
}

func createValueMessage() *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String("fake-origin-2"),
		EventType: events.Envelope_ValueMetric.Enum(),
		ValueMetric: &events.ValueMetric{
			Name:  proto.String("fake-metric-name"),
			Value: proto.Float64(42),
			Unit:  proto.String("fake-unit"),
		},
	}
}

func createCounterMessage(name string, origin string) *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String(origin),
		EventType: events.Envelope_CounterEvent.Enum(),
		CounterEvent: &events.CounterEvent{
			Name:  proto.String(name),
			Delta: proto.Uint64(4),
		},
	}
}

func expectCorrectCounterNameDeltaAndTotal(outputMessage *events.Envelope, name string, delta uint64, total uint64) {
	Expect(outputMessage.GetCounterEvent().GetName()).To(Equal(name))
	Expect(outputMessage.GetCounterEvent().GetDelta()).To(Equal(delta))
	Expect(outputMessage.GetCounterEvent().GetTotal()).To(Equal(total))
}
