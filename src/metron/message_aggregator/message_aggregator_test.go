package message_aggregator_test

import (
	"fmt"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/gogo/protobuf/proto"
	"metron/message_aggregator"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MessageAggregator", func() {
	var (
		inputChan         chan *events.Envelope
		outputChan        chan *events.Envelope
		runComplete       chan struct{}
		messageAggregator *message_aggregator.MessageAggregator
		originalTTL       time.Duration
	)

	BeforeEach(func() {
		inputChan = make(chan *events.Envelope, 10)
		outputChan = make(chan *events.Envelope, 10)
		runComplete = make(chan struct{})
		messageAggregator = message_aggregator.New(loggertesthelper.Logger())
		originalTTL = message_aggregator.MaxTTL

		go func() {
			messageAggregator.Run(inputChan, outputChan)
			close(runComplete)
		}()
	})

	AfterEach(func() {
		close(inputChan)
		Eventually(runComplete).Should(BeClosed())
		message_aggregator.MaxTTL = originalTTL
	})

	It("passes non-marshallable messages through", func() {
		inputMessage := &events.Envelope{}
		inputChan <- inputMessage

		outputMessage := <-outputChan
		Expect(outputMessage).To(Equal(inputMessage))
	})

	It("passes heartbeats through", func() {
		inputMessage := createHeartbeatMessage()
		inputChan <- inputMessage

		outputMessage := <-outputChan
		Expect(outputMessage).To(Equal(inputMessage))
	})

	Describe("counter processing", func() {
		It("sets the Total field on a CounterEvent ", func() {
			inputChan <- createCounterMessage("total", "fake-origin-4")

			outputMessage := <-outputChan
			Expect(outputMessage.GetEventType()).To(Equal(events.Envelope_CounterEvent))
			assertCorrectCounterNameDeltaAndTotal(outputMessage, "total", 4, 4)
		})

		It("accumulates Deltas for CounterEvents with the same name and origin", func() {
			inputChan <- createCounterMessage("total", "fake-origin-4")
			inputChan <- createCounterMessage("total", "fake-origin-4")

			_ = <-outputChan
			outputMessage := <-outputChan

			assertCorrectCounterNameDeltaAndTotal(outputMessage, "total", 4, 8)
		})

		It("accumulates differently-named counters separately", func() {
			inputChan <- createCounterMessage("total1", "fake-origin-4")
			inputChan <- createCounterMessage("total2", "fake-origin-4")

			outputMessage := <-outputChan
			assertCorrectCounterNameDeltaAndTotal(outputMessage, "total1", 4, 4)

			outputMessage = <-outputChan
			assertCorrectCounterNameDeltaAndTotal(outputMessage, "total2", 4, 4)
		})

		It("does not accumulate for counters when receiving a non-counter event", func() {
			inputChan <- createHeartbeatMessage()
			inputChan <- createCounterMessage("counter1", "fake-origin-4")
			outputMessage := <-outputChan

			Expect(outputMessage.GetEventType()).To(Equal(events.Envelope_Heartbeat))

			outputMessage = <-outputChan
			Expect(outputMessage.GetEventType()).To(Equal(events.Envelope_CounterEvent))
			assertCorrectCounterNameDeltaAndTotal(outputMessage, "counter1", 4, 4)
		})

		It("accumulates independently for different origins", func() {
			inputChan <- createCounterMessage("counter1", "fake-origin-4")
			inputChan <- createCounterMessage("counter1", "fake-origin-5")
			inputChan <- createCounterMessage("counter1", "fake-origin-4")
			outputMessage := <-outputChan

			Expect(outputMessage.GetOrigin()).To(Equal("fake-origin-4"))
			assertCorrectCounterNameDeltaAndTotal(outputMessage, "counter1", 4, 4)

			outputMessage = <-outputChan

			Expect(outputMessage.GetOrigin()).To(Equal("fake-origin-5"))
			assertCorrectCounterNameDeltaAndTotal(outputMessage, "counter1", 4, 4)

			outputMessage = <-outputChan

			Expect(outputMessage.GetOrigin()).To(Equal("fake-origin-4"))
			assertCorrectCounterNameDeltaAndTotal(outputMessage, "counter1", 4, 8)
		})
	})

	Context("single StartStop message", func() {
		var outputMessage *events.Envelope
		BeforeEach(func() {
			inputChan <- createStartMessage(123, events.PeerType_Client)
			inputChan <- createStopMessage(123, events.PeerType_Client)

			outputMessage = <-outputChan
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
		inputChan <- createStopMessage(123, events.PeerType_Client)
		Consistently(outputChan).ShouldNot(Receive())
	})

	Context("message expiry", func() {
		BeforeEach(func() {
			message_aggregator.MaxTTL = 0
		})

		It("does not send a combined event if the stop event doesn't arrive within the TTL", func() {
			inputChan <- createStartMessage(123, events.PeerType_Client)
			time.Sleep(1)
			inputChan <- createStopMessage(123, events.PeerType_Client)

			Consistently(outputChan).ShouldNot(Receive())
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
			inputChan <- createStartMessage(123, events.PeerType_Client)
			eventuallyExpectMetric("httpStartReceived", 1)
		})

		It("emits a HTTP stop counter", func() {
			inputChan <- createStopMessage(123, events.PeerType_Client)
			eventuallyExpectMetric("httpStopReceived", 1)
		})

		It("emits a HTTP StartStop counter", func() {
			inputChan <- createStartMessage(123, events.PeerType_Client)
			inputChan <- createStopMessage(123, events.PeerType_Client)
			eventuallyExpectMetric("httpStartStopEmitted", 1)
		})

		It("emits a counter for uncategorized events", func() {
			inputChan <- createHeartbeatMessage()
			eventuallyExpectMetric("uncategorizedEvents", 1)
		})

		It("emits a counter for unmatched start events", func() {
			message_aggregator.MaxTTL = 0
			inputChan <- createStartMessage(123, events.PeerType_Client)
			time.Sleep(1)
			inputChan <- createStartMessage(123, events.PeerType_Client)
			eventuallyExpectMetric("httpUnmatchedStartReceived", 1)
		})

		It("emits a counter for unmatched stop events", func() {
			inputChan <- createStopMessage(123, events.PeerType_Client)
			eventuallyExpectMetric("httpUnmatchedStopReceived", 1)
		})

		It("emits a counter for counter events", func() {
			inputChan <- createCounterMessage("counter1", "fake-origin-1")
			eventuallyExpectMetric("counterEventReceived", 1)

			// since we're counting counters, let's make sure we're not adding their deltas
			inputChan <- createCounterMessage("counter1", "fake-origin-1")
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

func createHeartbeatMessage() *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String("fake-origin-3"),
		EventType: events.Envelope_Heartbeat.Enum(),
		Heartbeat: factories.NewHeartbeat(1, 2, 3),
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

func assertCorrectCounterNameDeltaAndTotal(outputMessage *events.Envelope, name string, delta uint64, total uint64) {
	Expect(outputMessage.GetCounterEvent().GetName()).To(Equal(name))
	Expect(outputMessage.GetCounterEvent().GetDelta()).To(Equal(delta))
	Expect(outputMessage.GetCounterEvent().GetTotal()).To(Equal(total))
}
