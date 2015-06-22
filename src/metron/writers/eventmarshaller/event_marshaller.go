// Package eventmarshaller provides a tool for marshalling Envelopes
// to Protocol Buffer messages.
//
// Use
//
// Instantiate a Marshaller and run it:
//
//		marshaller := eventmarshaller.New(logger)
//		inputChan := make(chan *events.Envelope) // or use a channel provided by some other source
//		outputChan := make(chan []byte)
//		go marshaller.Run(inputChan, outputChan)
//
// The marshaller self-instruments, counting the number of messages
// processed and the number of errors. These can be accessed through the Emit
// function on the marshaller.

// TODO: Fix above comment block and similar comment block in eventunmarshaller
package eventmarshaller

import (
	"sync/atomic"
	"unicode"

	"metron/writers"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/davecgh/go-spew/spew"
	"github.com/gogo/protobuf/proto"
)

// A EventMarshaller is an self-instrumenting tool for converting dropsonde
// Envelopes to binary (Protocol Buffer) messages.
type EventMarshaller struct {
	logger            *gosteno.Logger
	outputWriter      writers.ByteArrayWriter
	messageCounts     map[events.Envelope_EventType]*uint64
	marshalErrorCount uint64
}

// New instantiates a EventMarshaller and logs to the provided logger.
func New(logger *gosteno.Logger, outputWriter writers.ByteArrayWriter) *EventMarshaller {
	messageCounts := make(map[events.Envelope_EventType]*uint64)
	for key := range events.Envelope_EventType_name {
		var count uint64
		messageCounts[events.Envelope_EventType(key)] = &count
	}
	return &EventMarshaller{
		logger:        logger,
		outputWriter:  outputWriter,
		messageCounts: messageCounts,
	}
}

func (u *EventMarshaller) Write(message *events.Envelope) {
	messageBytes, err := proto.Marshal(message)
	if err != nil {
		u.logger.Errorf("eventMarshaller: marshal error %v for message %v", err, message)
		incrementCount(&u.marshalErrorCount)
		return
	}

	u.logger.Debugf("eventMarshaller: marshalled message %v", spew.Sprintf("%v", message))

	u.incrementMessageCount(message.GetEventType())
	u.outputWriter.Write(messageBytes)
}

func (u *EventMarshaller) incrementMessageCount(eventType events.Envelope_EventType) {
	incrementCount(u.messageCounts[eventType])
}

func incrementCount(count *uint64) {
	atomic.AddUint64(count, 1)
}

func (m *EventMarshaller) metrics() []instrumentation.Metric {
	var metrics []instrumentation.Metric

	for eventType, eventName := range events.Envelope_EventType_name {
		modifiedEventName := []rune(eventName)
		modifiedEventName[0] = unicode.ToLower(modifiedEventName[0])
		metricName := string(modifiedEventName) + "Marshalled"

		metricValue := atomic.LoadUint64(m.messageCounts[events.Envelope_EventType(eventType)])
		metrics = append(metrics, instrumentation.Metric{Name: metricName, Value: metricValue})
	}

	metrics = append(metrics, instrumentation.Metric{
		Name:  "marshalErrors",
		Value: atomic.LoadUint64(&m.marshalErrorCount),
	})

	return metrics
}

// Emit returns the current metrics the EventMarshaller keeps about itself.
func (m *EventMarshaller) Emit() instrumentation.Context {
	return instrumentation.Context{
		Name:    "eventMarshaller",
		Metrics: m.metrics(),
	}
}
