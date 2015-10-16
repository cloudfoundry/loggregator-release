package integration_test

import (
	"net"
	"time"

	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/nu7hatch/gouuid"
	. "github.com/onsi/gomega"
)

func basicValueMetric(name string, value float64, unit string) *events.ValueMetric {
	return &events.ValueMetric{
		Name:  proto.String(name),
		Value: proto.Float64(value),
		Unit:  proto.String(unit),
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
	message, _ := proto.Marshal(basicValueMessageEnvelope())
	return message
}

func basicValueMessageEnvelope() *events.Envelope {
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

func basicHTTPStartMessage(requestId string) []byte {
	message, _ := proto.Marshal(basicHTTPStartMessageEnvelope(requestId))
	return message
}

func basicHTTPStartMessageEnvelope(requestId string) *events.Envelope {
	uuid, _ := uuid.ParseHex(requestId)
	return &events.Envelope{
		Origin:    proto.String("fake-origin-2"),
		EventType: events.Envelope_HttpStart.Enum(),
		HttpStart: &events.HttpStart{
			Timestamp:     proto.Int64(12),
			RequestId:     factories.NewUUID(uuid),
			PeerType:      events.PeerType_Client.Enum(),
			Method:        events.Method_GET.Enum(),
			Uri:           proto.String("some uri"),
			RemoteAddress: proto.String("some address"),
			UserAgent:     proto.String("some user agent"),
		},
	}
}

func basicHTTPStopMessage(requestId string) []byte {
	message, _ := proto.Marshal(basicHTTPStopMessageEnvelope(requestId))
	return message
}

func basicHTTPStopMessageEnvelope(requestId string) *events.Envelope {
	uuid, _ := uuid.ParseHex(requestId)
	return &events.Envelope{
		Origin:    proto.String("fake-origin-2"),
		EventType: events.Envelope_HttpStop.Enum(),
		HttpStop: &events.HttpStop{
			Timestamp:     proto.Int64(12),
			Uri:           proto.String("some uri"),
			RequestId:     factories.NewUUID(uuid),
			PeerType:      events.PeerType_Client.Enum(),
			StatusCode:    proto.Int32(404),
			ContentLength: proto.Int64(98475189),
		},
	}
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
		testServer, err = net.ListenPacket("udp4", address)
		return err
	}).Should(Succeed())

	return testServer
}
