package eventunmarshaller

import (
	"sync"
	"sync/atomic"
	"unicode"

	"metron/writers"

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
	logger                  *gosteno.Logger
	outputWriter            writers.EnvelopeWriter
	receiveCounts           map[events.Envelope_EventType]*uint64
	logMessageReceiveCounts map[string]*uint64
	unmarshalErrorCount     uint64
	lock                    sync.RWMutex
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
		logger:                  logger,
		outputWriter:            outputWriter,
		receiveCounts:           receiveCounts,
		logMessageReceiveCounts: make(map[string]*uint64),
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
		metrics.BatchIncrementCounter("EventUnmarshaller.unmarshalErrors")
		incrementCount(&u.unmarshalErrorCount)
		return nil, err
	}

	u.logger.Debugf("eventUnmarshaller: received message %v", spew.Sprintf("%v", envelope))

	if envelope.GetEventType() == events.Envelope_LogMessage {
		u.incrementLogMessageReceiveCount(envelope.GetLogMessage().GetAppId())
	} else {
		u.incrementReceiveCount(envelope.GetEventType())
	}

	return envelope, nil
}

func (u *EventUnmarshaller) incrementLogMessageReceiveCount(appID string) {
	metrics.BatchIncrementCounter("EventUnmarshaller.logMessageTotal")

	_, ok := u.logMessageReceiveCounts[appID]
	if ok == false {
		var count uint64
		u.lock.Lock()
		u.logMessageReceiveCounts[appID] = &count
		u.lock.Unlock()
	}
	incrementCount(u.logMessageReceiveCounts[appID])
	incrementCount(u.receiveCounts[events.Envelope_LogMessage])
}

func (u *EventUnmarshaller) incrementReceiveCount(eventType events.Envelope_EventType) {
	name, ok := events.Envelope_EventType_name[int32(eventType)]

	if !ok {
		var newCounter uint64
		u.receiveCounts[eventType] = &newCounter
		name = "unknownEventType"
	}

	modifiedEventName := []rune(name)
	modifiedEventName[0] = unicode.ToLower(modifiedEventName[0])
	metricName := string(modifiedEventName) + "Received"

	metrics.BatchIncrementCounter("EventUnmarshaller." + metricName)

	incrementCount(u.receiveCounts[eventType])
}

func incrementCount(count *uint64) {
	atomic.AddUint64(count, 1)
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

	return metrics
}

// Emit returns the current metrics the DropsondeMarshaller keeps about itself.
func (u *EventUnmarshaller) Emit() instrumentation.Context {
	return instrumentation.Context{
		Name:    "EventUnmarshaller",
		Metrics: u.metrics(),
	}
}
