package eventmarshaller

import (
	"unicode"

	"metron/writers"

	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/davecgh/go-spew/spew"
	"github.com/gogo/protobuf/proto"
)

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
	modifiedEventName := []rune(eventType.String())
	modifiedEventName[0] = unicode.ToLower(modifiedEventName[0])
	metricName := string(modifiedEventName) + "Marshalled"
	metrics.BatchIncrementCounter("dropsondeMarshaller." + metricName)
}
