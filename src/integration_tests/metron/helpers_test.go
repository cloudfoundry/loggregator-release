package integration_test

import (
	"net"
	"time"

	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/gomega"
)

func basicValueMetric(name string, value float64, unit string) *events.ValueMetric {
	return &events.ValueMetric{
		Name:  proto.String(name),
		Value: proto.Float64(value),
		Unit:  proto.String(unit),
	}
}

func basicHeartbeatMessage() []byte {
	message, _ := proto.Marshal(basicHeartbeatEvent())

	return message
}

func basicHeartbeatEvent() *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String("fake-origin-1"),
		EventType: events.Envelope_Heartbeat.Enum(),
		Heartbeat: &events.Heartbeat{
			SentCount:     proto.Uint64(100),
			ReceivedCount: proto.Uint64(250),
			ErrorCount:    proto.Uint64(50),
		},
	}
}

func addDefaultTags(envelope *events.Envelope) *events.Envelope {
	envelope.Deployment = proto.String("deployment-name")
	envelope.Job = proto.String("test-component")
	envelope.Index = proto.String("42")
	envelope.Ip = proto.String(localIPAddress)

	return envelope
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

func eventsLogMessage(appID int, message string, timestamp time.Time) *events.Envelope {
	return &events.Envelope{
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
