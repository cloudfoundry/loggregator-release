// Package marshallingeventwriter provides a tool for marshalling Envelopes
// to Protocol Buffer messages.
//
// Use
//
// Instantiate a Marshaller and run it:
//
//		marshaller := marshallingeventwriter.NewMarshallingEventWriter(logger)
//		inputChan := make(chan *events.Envelope) // or use a channel provided by some other source
//		outputChan := make(chan []byte)
//		go marshaller.Run(inputChan, outputChan)
//
// The marshaller self-instruments, counting the number of messages
// processed and the number of errors. These can be accessed through the Emit
// function on the marshaller.

// TODO: Fix above comment block and similar comment block in unmarshallingeventwriter
package marshallingeventwriter

import (
	"io"
	"sync/atomic"
	"unicode"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/davecgh/go-spew/spew"
	"github.com/gogo/protobuf/proto"
)

// A MarshallingEventWriter is an self-instrumenting tool for converting dropsonde
// Envelopes to binary (Protocol Buffer) messages.
type MarshallingEventWriter struct {
	logger            *gosteno.Logger
	outputWriter	  io.Writer
	messageCounts     map[events.Envelope_EventType]*uint64
	marshalErrorCount uint64
}

// NewMarshallingEventWriter instantiates a MarshallingEventWriter and logs to the
// provided logger.
func NewMarshallingEventWriter(logger *gosteno.Logger, outputWriter io.Writer) *MarshallingEventWriter {
	messageCounts := make(map[events.Envelope_EventType]*uint64)
	for key := range events.Envelope_EventType_name {
		var count uint64
		messageCounts[events.Envelope_EventType(key)] = &count
	}
	return &MarshallingEventWriter{
		logger:        logger,
		outputWriter:  outputWriter,
		messageCounts: messageCounts,
	}
}

func (u *MarshallingEventWriter) Write(message *events.Envelope) {
	messageBytes, err := proto.Marshal(message)
	if err != nil {
		u.logger.Errorf("marshallingEventWriter: marshal error %v for message %v", err, message)
		incrementCount(&u.marshalErrorCount)
		return
	}

	u.logger.Debugf("marshallingEventWriter: marshalled message %v", spew.Sprintf("%v", message))

	u.incrementMessageCount(message.GetEventType())
	u.outputWriter.Write(messageBytes)
}

func (u *MarshallingEventWriter) incrementMessageCount(eventType events.Envelope_EventType) {
	incrementCount(u.messageCounts[eventType])
}

func incrementCount(count *uint64) {
	atomic.AddUint64(count, 1)
}

func (m *MarshallingEventWriter) metrics() []instrumentation.Metric {
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

// Emit returns the current metrics the MarshallingEventWriter keeps about itself.
func (m *MarshallingEventWriter) Emit() instrumentation.Context {
	return instrumentation.Context{
		Name:    "marshallingEventWriter",
		Metrics: m.metrics(),
	}
}
