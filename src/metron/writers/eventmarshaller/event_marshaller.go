package eventmarshaller

import (
	"unicode"
	"unicode/utf8"

	"metron/writers"

	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/davecgh/go-spew/spew"
	"github.com/gogo/protobuf/proto"
)

var metricNames map[events.Envelope_EventType]string

func init() {
	metricNames = make(map[events.Envelope_EventType]string)
	for eventType, eventName := range events.Envelope_EventType_name {
		r, n := utf8.DecodeRuneInString(eventName)
		modifiedName := string(unicode.ToLower(r)) + eventName[n:]
		metricName := "dropsondeMarshaller." + modifiedName + "Marshalled"
		metricNames[events.Envelope_EventType(eventType)] = metricName
	}
}

// A EventMarshaller is an self-instrumenting tool for converting dropsonde
// Envelopes to binary (Protocol Buffer) messages.
type EventMarshaller struct {
	logger       *gosteno.Logger
	outputWriter writers.ByteArrayWriter
}

// New instantiates a EventMarshaller and logs to the provided logger.
func New(outputWriter writers.ByteArrayWriter, logger *gosteno.Logger) *EventMarshaller {
	return &EventMarshaller{
		logger:       logger,
		outputWriter: outputWriter,
	}
}

func (m *EventMarshaller) Write(message *events.Envelope) {
	messageBytes, err := proto.Marshal(message)
	if err != nil {
		m.logger.Errorf("eventMarshaller: marshal error %v for message %v", err, message)
		metrics.BatchIncrementCounter("dropsondeMarshaller.marshalErrors")
		return
	}

	m.logger.Debugf("eventMarshaller: marshalled message %v", spew.Sprintf("%v", message))
	m.incrementMessageCount(message.GetEventType())
	m.outputWriter.Write(messageBytes)
}

func (m *EventMarshaller) incrementMessageCount(eventType events.Envelope_EventType) {
	metricName := metricNames[eventType]
	metrics.BatchIncrementCounter(metricName)
}
