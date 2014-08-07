package message_aggregator_test

import (
	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"metron/message_aggregator"
	"time"
)

var _ = Describe("MessageAggregator", func() {
	var (
		inputChan         chan []byte
		outputChan        chan []byte
		runComplete       chan struct{}
		messageAggregator message_aggregator.MessageAggregator
		originalTTL       time.Duration
	)

	BeforeEach(func() {
		inputChan = make(chan []byte, 10)
		outputChan = make(chan []byte, 10)
		runComplete = make(chan struct{})
		messageAggregator = message_aggregator.NewMessageAggregator(loggertesthelper.Logger())
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
		inputMessage := []byte{1, 2, 3}
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

	Context("single StartStop message", func() {
		var outputMessage events.Envelope
		BeforeEach(func() {
			inputChan <- createStartMessage(123, events.PeerType_Client)
			inputChan <- createStopMessage(123, events.PeerType_Client)

			outputMessageBytes := <-outputChan
			proto.Unmarshal(outputMessageBytes, &outputMessage)
		})

		It("aggregates HTTP start+stop messages", func() {
			Expect(outputMessage.GetEventType()).To(Equal(events.Envelope_HttpStartStop))
		})

		It("populates all fields in the StartStop message correctly", func() {
			Expect(&outputMessage).To(Equal(createStartStopMessage(123, events.PeerType_Client)))
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
			}).Should(Equal(value))
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

		It("emits an unmarshal error counter", func() {
			inputChan <- []byte{1, 2, 3}
			eventuallyExpectMetric("unmarshalErrors", 1)
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
	})
})

func createStartMessage(requestId uint64, peerType events.PeerType) []byte {
	message, _ := proto.Marshal(&events.Envelope{
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
			ApplicationId: &events.UUID{
				Low:  proto.Uint64(4),
				High: proto.Uint64(5),
			},
			InstanceIndex: proto.Int32(6),
			InstanceId:    proto.String("fake-instance-id-1"),
		},
	})

	return message
}

func createStopMessage(requestId uint64, peerType events.PeerType) []byte {
	message, _ := proto.Marshal(&events.Envelope{
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
	})

	return message
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
				Low:  proto.Uint64(4),
				High: proto.Uint64(5),
			},
			InstanceIndex: proto.Int32(6),
			InstanceId:    proto.String("fake-instance-id-1"),
		},
	}
}

func createHeartbeatMessage() []byte {
	message, _ := proto.Marshal(&events.Envelope{
		Origin:    proto.String("fake-origin-3"),
		EventType: events.Envelope_Heartbeat.Enum(),
		Heartbeat: factories.NewHeartbeat(1, 2, 3),
	})
	return message
}
