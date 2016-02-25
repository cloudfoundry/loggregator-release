package eventtypereader

import (
	"tools/benchmark/metricsreporter"

	"github.com/cloudfoundry/sonde-go/events"
)

type MessageReader interface {
	Read() *events.Envelope
	Close()
}

type EventTypeReader struct {
	receivedCounter *metricsreporter.Counter
	reader          MessageReader
	eventType       events.Envelope_EventType
	origin          string
}

func New(receivedCounter *metricsreporter.Counter, reader MessageReader, typ events.Envelope_EventType, origin string) *EventTypeReader {
	return &EventTypeReader{
		receivedCounter: receivedCounter,
		reader:          reader,
		eventType:       typ,
		origin:          origin,
	}
}

func (lr *EventTypeReader) Read() {
	message := lr.reader.Read()
	if message != nil && message.GetEventType() == lr.eventType && message.GetOrigin() == lr.origin {
		lr.receivedCounter.IncrementValue()
	}
}

func (lr *EventTypeReader) Close() {
	lr.reader.Close()
}
