package integration_test

import (
	"net"
	"time"

	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/gomega"
)

func basicValueMetric() *events.ValueMetric {
	return &events.ValueMetric{
		Name:  proto.String("test.gauge"),
		Value: proto.Float64(23.0),
		Unit:  proto.String("gauge"),
	}
}

func basicHeartbeatMessage() []byte {
	peerType := events.PeerType_Server
	message, _ := proto.Marshal(&events.Envelope{
		Origin:    proto.String("fake-origin-1"),
		EventType: events.Envelope_Heartbeat.Enum(),
		HttpStart: &events.HttpStart{
			Timestamp: proto.Int64(1),
			RequestId: &events.UUID{
				Low:  proto.Uint64(0),
				High: proto.Uint64(1),
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

func basicValueMessage() []byte {
	message, _ := proto.Marshal(&events.Envelope{
		Origin:    proto.String("fake-origin-2"),
		EventType: events.Envelope_ValueMetric.Enum(),
		ValueMetric: &events.ValueMetric{
			Name:  proto.String("fake-metric-name"),
			Value: proto.Float64(42),
			Unit:  proto.String("fake-unit"),
		},
	})

	return message
}

func basicCounterEventMessage() []byte {
	message, _ := proto.Marshal(&events.Envelope{
		Origin:    proto.String("fake-origin-2"),
		EventType: events.Envelope_CounterEvent.Enum(),
		CounterEvent: &events.CounterEvent{
			Name:  proto.String("fake-counter-event-name"),
			Delta: proto.Uint64(1),
		},
	})

	return message
}

func getMetricFromContext(context *instrumentation.Context, name string) *instrumentation.Metric {
	for _, metric := range context.Metrics {
		if metric.Name == name {
			return &metric
		}
	}
	return nil
}

func legacyLogMessage(appID int, message string, timestamp time.Time) []byte {
	envelope := &logmessage.LogEnvelope{
		RoutingKey: proto.String("fake-routing-key"),
		Signature:  []byte{1, 2, 3},
		LogMessage: &logmessage.LogMessage{
			Message:     []byte(message),
			MessageType: logmessage.LogMessage_OUT.Enum(),
			Timestamp:   proto.Int64(timestamp.UnixNano()),
			AppId:       proto.String(string(appID)),
			SourceName:  proto.String("fake-source-id"),
			SourceId:    proto.String("fake-source-id"),
		},
	}

	bytes, _ := proto.Marshal(envelope)
	return bytes
}

func eventsLogMessage(appID int, message string, timestamp time.Time) []byte {
	envelope := &events.Envelope{
		Origin:    proto.String("legacy"),
		EventType: events.Envelope_LogMessage.Enum(),
		LogMessage: &events.LogMessage{
			Message:        []byte(message),
			MessageType:    events.LogMessage_OUT.Enum(),
			Timestamp:      proto.Int64(timestamp.UnixNano()),
			AppId:          proto.String(string(appID)),
			SourceType:     proto.String("fake-source-id"),
			SourceInstance: proto.String("fake-source-id"),
		},
	}

	bytes, _ := proto.Marshal(envelope)
	return bytes
}

func eventuallyListensForUDP(address string) net.PacketConn {
	var testServer net.PacketConn

	Eventually(func() error {
		var err error
		testServer, err = net.ListenPacket("udp", address)
		return err
	}).Should(Succeed())

	return testServer
}
