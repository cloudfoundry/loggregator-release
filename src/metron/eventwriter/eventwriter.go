package eventwriter

import (
	"errors"
	"metron/writers"
	"sync"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/sonde-go/events"
)

type EventWriter struct {
	origin string
	writer writers.EnvelopeWriter
	lock   sync.RWMutex
}

func New(origin string) *EventWriter {
	return &EventWriter{
		origin: origin,
	}
}

func (e *EventWriter) Emit(event events.Event) error {
	envelope, err := emitter.Wrap(event, e.origin)
	if err != nil {
		return err
	}

	return e.EmitEnvelope(envelope)
}

func (e *EventWriter) EmitEnvelope(envelope *events.Envelope) error {
	e.lock.RLock()
	defer e.lock.RUnlock()
	if e.writer == nil {
		return errors.New("EventWriter: No envelope writer set (see SetWriter)")
	}
	e.writer.Write(envelope)
	return nil
}

func (e *EventWriter) Origin() string {
	return e.origin
}

func (e *EventWriter) SetWriter(writer writers.EnvelopeWriter) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.writer = writer
}
