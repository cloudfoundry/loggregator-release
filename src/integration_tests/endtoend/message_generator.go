package endtoend

import (
	"time"

	"code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2"
	"github.com/cloudfoundry/sonde-go/events"
	"google.golang.org/protobuf/proto"
)

type ValueMetricGenerator struct{}

func NewValueMetricGenerator() *ValueMetricGenerator {
	return &ValueMetricGenerator{}
}

func (*ValueMetricGenerator) Generate() []byte {
	return BasicValueMetric()
}

func BasicValueMetric() []byte {
	message, _ := proto.Marshal(BasicValueMetricEnvelope("test-origin"))
	return message
}

func BasicValueMetricEnvelope(origin string) *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String(origin),
		EventType: events.Envelope_ValueMetric.Enum(),
		ValueMetric: &events.ValueMetric{
			Name:  proto.String("fake-metric-name"),
			Value: proto.Float64(42),
			Unit:  proto.String("fake-unit"),
		},
	}
}

type LogMessageGenerator struct {
	appID string
}

func NewLogMessageGenerator(appID string) *LogMessageGenerator {
	return &LogMessageGenerator{
		appID: appID,
	}
}

func (l *LogMessageGenerator) Generate() []byte {
	message, _ := proto.Marshal(BasicLogMessageEnvelope("test-origin", l.appID))
	return message
}

func BasicCounterEvent(origin string) *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String(origin),
		EventType: events.Envelope_CounterEvent.Enum(),
		CounterEvent: &events.CounterEvent{
			Name:  proto.String("fake-counter-event"),
			Delta: proto.Uint64(1),
			Total: proto.Uint64(2),
		},
	}
}

func BasicLogMessageEnvelope(origin string, appID string) *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String(origin),
		EventType: events.Envelope_LogMessage.Enum(),
		LogMessage: &events.LogMessage{
			Message:     []byte("test message"),
			MessageType: events.LogMessage_OUT.Enum(),
			Timestamp:   proto.Int64(time.Now().UnixNano()),
			AppId:       proto.String(appID),
		},
	}
}

func BasicLogMessageEnvelopeV2(appID string) *loggregator_v2.Envelope {
	return &loggregator_v2.Envelope{
		Timestamp:  time.Now().UnixNano(),
		SourceId:   appID,
		InstanceId: "0",
		Message: &loggregator_v2.Envelope_Log{
			Log: &loggregator_v2.Log{
				Payload: []byte("log message"),
			},
		},
	}
}
