package eventunmarshaller

import (
	"unicode"

	"metron/writers"

	"fmt"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/davecgh/go-spew/spew"
	"github.com/gogo/protobuf/proto"
)

// An EventUnmarshaller is an self-instrumenting tool for converting Protocol
// Buffer-encoded dropsonde messages to Envelope instances.
type EventUnmarshaller struct {
	logger       *gosteno.Logger
	outputWriter writers.EnvelopeWriter
}

// New instantiates a EventUnmarshaller and logs to the
// provided logger.
func New(outputWriter writers.EnvelopeWriter, logger *gosteno.Logger) *EventUnmarshaller {
	return &EventUnmarshaller{
		logger:       logger,
		outputWriter: outputWriter,
	}
}

func (u *EventUnmarshaller) Write(message []byte) {
	envelope, err := u.UnmarshallMessage(message)
	if err != nil {
		return
	}
	u.outputWriter.Write(envelope)
}

func (u *EventUnmarshaller) UnmarshallMessage(message []byte) (*events.Envelope, error) {
	envelope := &events.Envelope{}
	err := proto.Unmarshal(message, envelope)
	if err != nil {
		u.logger.Debugf("eventUnmarshaller: unmarshal error %v for message %v", err, message)
		metrics.BatchIncrementCounter("dropsondeUnmarshaller.unmarshalErrors")
		return nil, err
	}

	u.logger.Debugf("eventUnmarshaller: received message %v", spew.Sprintf("%v", envelope))

	if isUnknownEventType(envelope.EventType) {
		metrics.BatchIncrementCounter("dropsondeUnmarshaller.unknownEventTypeReceived")
		u.logger.Debugf("eventUnmarshaller: received unknown event type %#v", envelope.EventType)
		return nil, fmt.Errorf("eventUnmarshaller: received unknown event type %#v", envelope.EventType)
	}

	u.incrementReceiveCount(envelope.GetEventType())

	return envelope, nil
}

func (u *EventUnmarshaller) incrementReceiveCount(eventType events.Envelope_EventType) {
	switch eventType {
	case events.Envelope_LogMessage:
		// LogMessage is a special case. `logMessageReceived` used to be broken out by app ID, and
		// `logMessageTotal` was the sum of all of those.
		metrics.BatchIncrementCounter("dropsondeUnmarshaller.logMessageTotal")
	default:
		name := eventType.String()
		modifiedEventName := []rune(name)
		modifiedEventName[0] = unicode.ToLower(modifiedEventName[0])
		metricName := string(modifiedEventName) + "Received"

		metrics.BatchIncrementCounter("dropsondeUnmarshaller." + metricName)
	}
}

func isUnknownEventType(eventType *events.Envelope_EventType) bool {
	if eventType == nil {
		return true
	}

	if _, ok := events.Envelope_EventType_name[int32(*eventType)]; !ok {
		return true
	}

	return false
}
