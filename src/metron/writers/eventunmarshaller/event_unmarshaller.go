package eventunmarshaller

import (
	"errors"
	"unicode"
	"unicode/utf8"

	"metron/writers"

	"fmt"

	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/davecgh/go-spew/spew"
	"github.com/gogo/protobuf/proto"
)

var invalidEnvelope = errors.New("Invalid Envelope")
var metricNames map[events.Envelope_EventType]string

func init() {
	metricNames = make(map[events.Envelope_EventType]string)
	for eventType, eventName := range events.Envelope_EventType_name {
		r, n := utf8.DecodeRuneInString(eventName)
		modifiedName := string(unicode.ToLower(r)) + eventName[n:]
		metricName := "dropsondeUnmarshaller." + modifiedName + "Received"
		metricNames[events.Envelope_EventType(eventType)] = metricName
	}
}

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

	if !valid(envelope) {
		u.logger.Debugf("eventUnmarshaller: validation failed for message %v", message)
		metrics.BatchIncrementCounter("dropsondeUnmarshaller.unmarshalErrors")
		return nil, invalidEnvelope
	}

	u.logger.Debugf("eventUnmarshaller: received message %v", spew.Sprintf("%v", envelope))

	if err := u.incrementReceiveCount(envelope.GetEventType()); err != nil {
		u.logger.Debug(err.Error())
		return nil, err
	}

	return envelope, nil
}

func (u *EventUnmarshaller) incrementReceiveCount(eventType events.Envelope_EventType) error {
	var err error
	switch eventType {
	case events.Envelope_LogMessage:
		// LogMessage is a special case. `logMessageReceived` used to be broken out by app ID, and
		// `logMessageTotal` was the sum of all of those.
		metrics.BatchIncrementCounter("dropsondeUnmarshaller.logMessageTotal")
	default:
		metricName := metricNames[eventType]
		if metricName == "" {
			metricName = "dropsondeUnmarshaller.unknownEventTypeReceived"
			err = fmt.Errorf("eventUnmarshaller: received unknown event type %#v", eventType)
		}
		metrics.BatchIncrementCounter(metricName)
	}

	return err
}

func valid(env *events.Envelope) bool {
	switch env.GetEventType() {
	case events.Envelope_HttpStart:
		return env.GetHttpStart() != nil
	case events.Envelope_HttpStop:
		return env.GetHttpStop() != nil
	case events.Envelope_HttpStartStop:
		return env.GetHttpStartStop() != nil
	case events.Envelope_LogMessage:
		return env.GetLogMessage() != nil
	case events.Envelope_ValueMetric:
		return env.GetValueMetric() != nil
	case events.Envelope_CounterEvent:
		return env.GetCounterEvent() != nil
	case events.Envelope_Error:
		return env.GetError() != nil
	case events.Envelope_ContainerMetric:
		return env.GetContainerMetric() != nil
	}
	return true
}
