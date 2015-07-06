package eventunmarshaller

import (
	"sync"
	"sync/atomic"
	"unicode"

	"metron/writers"

	"fmt"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/davecgh/go-spew/spew"
	"github.com/gogo/protobuf/proto"
)

// An EventUnmarshaller is an self-instrumenting tool for converting Protocol
// Buffer-encoded dropsonde messages to Envelope instances.
type EventUnmarshaller struct {
	logger                *gosteno.Logger
	outputWriter          writers.EnvelopeWriter
	receiveCounts         map[events.Envelope_EventType]*uint64
	unknownEventTypeCount uint64
	unmarshalErrorCount   uint64
	lock                  sync.RWMutex
}

// New instantiates a EventUnmarshaller and logs to the
// provided logger.
func New(outputWriter writers.EnvelopeWriter, logger *gosteno.Logger) *EventUnmarshaller {
	receiveCounts := make(map[events.Envelope_EventType]*uint64)
	for key := range events.Envelope_EventType_name {
		var count uint64
		receiveCounts[events.Envelope_EventType(key)] = &count
	}

	return &EventUnmarshaller{
		logger:        logger,
		outputWriter:  outputWriter,
		receiveCounts: receiveCounts,
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
		incrementCount(&u.unmarshalErrorCount)
		return nil, err
	}

	u.logger.Debugf("eventUnmarshaller: received message %v", spew.Sprintf("%v", envelope))

	if isUnknownEventType(envelope.EventType) {
		metrics.BatchIncrementCounter("dropsondeUnmarshaller.unknownEventTypeReceived")
		incrementCount(&u.unknownEventTypeCount)
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
		incrementCount(u.receiveCounts[events.Envelope_LogMessage])
	default:
		name := eventType.String()
		modifiedEventName := []rune(name)
		modifiedEventName[0] = unicode.ToLower(modifiedEventName[0])
		metricName := string(modifiedEventName) + "Received"

		metrics.BatchIncrementCounter("dropsondeUnmarshaller." + metricName)
		incrementCount(u.receiveCounts[eventType])
	}
}

func incrementCount(count *uint64) {
	atomic.AddUint64(count, 1)
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

func (u *EventUnmarshaller) metrics() []instrumentation.Metric {
	var metrics []instrumentation.Metric

	u.lock.RLock()

	metricValue := atomic.LoadUint64(u.receiveCounts[events.Envelope_LogMessage])
	metrics = append(metrics, instrumentation.Metric{Name: logMessageTotal, Value: metricValue})

	u.lock.RUnlock()

	for eventType, counterPointer := range u.receiveCounts {
		if eventType == events.Envelope_LogMessage {
			continue
		}
		modifiedEventName := []rune(eventType.String())
		modifiedEventName[0] = unicode.ToLower(modifiedEventName[0])
		metricName := string(modifiedEventName) + "Received"
		metricValue := atomic.LoadUint64(counterPointer)
		metrics = append(metrics, instrumentation.Metric{Name: metricName, Value: metricValue})
	}

	metrics = append(metrics, instrumentation.Metric{
		Name:  unmarshalErrors,
		Value: atomic.LoadUint64(&u.unmarshalErrorCount),
	})

	metrics = append(metrics, instrumentation.Metric{
		Name:  unknownEvents,
		Value: atomic.LoadUint64(&u.unknownEventTypeCount),
	})

	return metrics
}

// Emit returns the current metrics the DropsondeMarshaller keeps about itself.
func (u *EventUnmarshaller) Emit() instrumentation.Context {
	return instrumentation.Context{
		Name:    "DropsondeUnmarshaller",
		Metrics: u.metrics(),
	}
}
