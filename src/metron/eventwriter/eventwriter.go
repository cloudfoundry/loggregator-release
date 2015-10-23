package eventwriter

import (
	"metron/writers"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/sonde-go/events"
)

type EventWriter struct {
	origin string
	writer writers.EnvelopeWriter
}

func New(origin string, writer writers.EnvelopeWriter) *EventWriter {
	return &EventWriter{
		origin: origin,
		writer: writer,
	}
}

func (e *EventWriter) Emit(event events.Event) error {
	envelope, err := emitter.Wrap(event, e.origin)
	if err != nil {
		return err
	}

	e.writer.Write(envelope)
	return nil
}

func (e *EventWriter) Close() {}
