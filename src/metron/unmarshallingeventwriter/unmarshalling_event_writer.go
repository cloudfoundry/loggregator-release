// Package unmarshallingeventwriter provides a tool for unmarshalling Envelopes
// from Protocol Buffer messages.
//
// Use
//
// Instantiate a Marshaller and run it:
//
//		unmarshaller := unmarshallingeventwriter.NewUnmarshallingEventWriter(logger)
//		inputChan :=  make(chan []byte) // or use a channel provided by some other source
//		outputChan := make(chan *events.Envelope)
//		go unmarshaller.Run(inputChan, outputChan)
//
// The unmarshaller self-instruments, counting the number of messages
// processed and the number of errors. These can be accessed through the Emit
// function on the unmarshaller.
package unmarshallingeventwriter

import (
	"sync"
	"sync/atomic"
	"unicode"

	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/davecgh/go-spew/spew"
	"github.com/gogo/protobuf/proto"
	"metron/envelopewriter"
)

// An UnmarshallingEventWriter is an self-instrumenting tool for converting Protocol
// Buffer-encoded dropsonde messages to Envelope instances.
type UnmarshallingEventWriter struct {
	logger                  *gosteno.Logger
	outputWriter            envelopewriter.EnvelopeWriter
	receiveCounts           map[events.Envelope_EventType]*uint64
	logMessageReceiveCounts map[string]*uint64
	unmarshalErrorCount     uint64
	sync.RWMutex
}

// NewUnmarshallingEventWriter instantiates a UnmarshallingEventWriter and logs to the
// provided logger.
func NewUnmarshallingEventWriter(logger *gosteno.Logger, outputWriter envelopewriter.EnvelopeWriter) *UnmarshallingEventWriter {
	receiveCounts := make(map[events.Envelope_EventType]*uint64)
	for key := range events.Envelope_EventType_name {
		var count uint64
		receiveCounts[events.Envelope_EventType(key)] = &count
	}

	return &UnmarshallingEventWriter{
		logger:                  logger,
		outputWriter:            outputWriter,
		receiveCounts:           receiveCounts,
		logMessageReceiveCounts: make(map[string]*uint64),
	}
}

func (u *UnmarshallingEventWriter) Write(message []byte) (bytesWritten int, err error) {
	envelope, err := u.UnmarshallMessage(message)
	if err != nil {
		return 0, err
	}
	u.outputWriter.Write(envelope)
	return len(message), nil
}

func (u *UnmarshallingEventWriter) UnmarshallMessage(message []byte) (*events.Envelope, error) {
	envelope := &events.Envelope{}
	err := proto.Unmarshal(message, envelope)
	if err != nil {
		u.logger.Debugf("unmarshallingEventWriter: unmarshal error %v for message %v", err, message)
		metrics.BatchIncrementCounter("UnmarshallingEventWriter.unmarshalErrors")
		incrementCount(&u.unmarshalErrorCount)
		return nil, err
	}

	u.logger.Debugf("unmarshallingEventWriter: received message %v", spew.Sprintf("%v", envelope))

	if envelope.GetEventType() == events.Envelope_LogMessage {
		u.incrementLogMessageReceiveCount(envelope.GetLogMessage().GetAppId())
	} else {
		u.incrementReceiveCount(envelope.GetEventType())
	}

	return envelope, nil
}

func (u *UnmarshallingEventWriter) incrementLogMessageReceiveCount(appID string) {
	metrics.BatchIncrementCounter("UnmarshallingEventWriter.logMessageTotal")

	_, ok := u.logMessageReceiveCounts[appID]
	if ok == false {
		var count uint64
		u.Lock()
		u.logMessageReceiveCounts[appID] = &count
		u.Unlock()
	}
	incrementCount(u.logMessageReceiveCounts[appID])
	incrementCount(u.receiveCounts[events.Envelope_LogMessage])
}

func (u *UnmarshallingEventWriter) incrementReceiveCount(eventType events.Envelope_EventType) {
	name, ok := events.Envelope_EventType_name[int32(eventType)]

	if !ok {
		var newCounter uint64
		u.receiveCounts[eventType] = &newCounter
		name = "unknownEventType"
	}

	modifiedEventName := []rune(name)
	modifiedEventName[0] = unicode.ToLower(modifiedEventName[0])
	metricName := string(modifiedEventName) + "Received"

	metrics.BatchIncrementCounter("UnmarshallingEventWriter." + metricName)

	incrementCount(u.receiveCounts[eventType])
}

func incrementCount(count *uint64) {
	atomic.AddUint64(count, 1)
}

func (u *UnmarshallingEventWriter) metrics() []instrumentation.Metric {
	var metrics []instrumentation.Metric

	u.RLock()

	metricValue := atomic.LoadUint64(u.receiveCounts[events.Envelope_LogMessage])
	metrics = append(metrics, instrumentation.Metric{Name: logMessageTotal, Value: metricValue})

	u.RUnlock()

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
func (u *UnmarshallingEventWriter) Emit() instrumentation.Context {
	return instrumentation.Context{
		Name:    "UnmarshallingEventWriter",
		Metrics: u.metrics(),
	}
}
