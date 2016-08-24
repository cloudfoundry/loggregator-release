package eventunmarshaller

import (
	"errors"
	"unicode"
	"unicode/utf8"

	"metron/writers"

	"fmt"

	"github.com/cloudfoundry/dropsonde/logging"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

var (
	invalidEnvelope = errors.New("Invalid Envelope")
	metricNames     map[events.Envelope_EventType]string
)

func init() {
	metricNames = make(map[events.Envelope_EventType]string)
	for eventType, eventName := range events.Envelope_EventType_name {
		r, n := utf8.DecodeRuneInString(eventName)
		modifiedName := string(unicode.ToLower(r)) + eventName[n:]
		metricName := "dropsondeUnmarshaller." + modifiedName + "Received"
		metricNames[events.Envelope_EventType(eventType)] = metricName
	}
}

//go:generate hel --type EventBatcher --output mock_event_batcher_test.go

type EventBatcher interface {
	BatchCounter(name string) (chainer metricbatcher.BatchCounterChainer)
	BatchIncrementCounter(name string)
}

// An EventUnmarshaller is an self-instrumenting tool for converting Protocol
// Buffer-encoded dropsonde messages to Envelope instances.
type EventUnmarshaller struct {
	logger       *gosteno.Logger
	outputWriter writers.EnvelopeWriter
	batcher      EventBatcher
}

// New instantiates a EventUnmarshaller and logs to the
// provided logger.
func New(outputWriter writers.EnvelopeWriter, batcher EventBatcher, logger *gosteno.Logger) *EventUnmarshaller {
	return &EventUnmarshaller{
		logger:       logger,
		outputWriter: outputWriter,
		batcher:      batcher,
	}
}

func (u *EventUnmarshaller) Write(message []byte) {
	envelope, err := u.UnmarshallMessage(message)
	if err != nil {
		u.logger.Errorf("Error unmarshalling: %s", err)
		return
	}
	u.outputWriter.Write(envelope)
}

func (u *EventUnmarshaller) UnmarshallMessage(message []byte) (*events.Envelope, error) {
	envelope := &events.Envelope{}
	err := proto.Unmarshal(message, envelope)
	if err != nil {
		u.logger.Errorf("eventUnmarshaller: unmarshal error %v", err)
		u.batcher.BatchIncrementCounter("dropsondeUnmarshaller.unmarshalErrors")
		return nil, err
	}

	if !valid(envelope) {
		logging.Debugf(u.logger, "eventUnmarshaller: validation failed for message %v", envelope.GetEventType())
		u.batcher.BatchIncrementCounter("dropsondeUnmarshaller.unmarshalErrors")
		return nil, invalidEnvelope
	}

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
		u.batcher.BatchIncrementCounter("dropsondeUnmarshaller.logMessageTotal")
	default:
		metricName := metricNames[eventType]
		if metricName == "" {
			metricName = "dropsondeUnmarshaller.unknownEventTypeReceived"
			err = fmt.Errorf("eventUnmarshaller: received unknown event type %#v", eventType)
		}
		u.batcher.BatchIncrementCounter(metricName)
	}

	u.batcher.BatchCounter("dropsondeUnmarshaller.receivedEnvelopes").
		SetTag("protocol", "udp").
		SetTag("event_type", eventType.String()).
		Increment()

	return err
}

func valid(env *events.Envelope) bool {
	switch env.GetEventType() {
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
