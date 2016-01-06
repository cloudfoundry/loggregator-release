package truncatingbuffer

import (
	"github.com/cloudfoundry/dropsonde/envelope_extensions"
	"github.com/cloudfoundry/sonde-go/events"
)

type BufferContext interface {
	EventAllowed(events.Envelope_EventType) bool
	Destination() string
	Origin() string
	AppID(*events.Envelope) string
}

type DefaultContext struct {
	destination string
	origin      string
}

func NewDefaultContext(origin string, destination string) *DefaultContext {
	return &DefaultContext{
		destination: destination,
		origin:      origin,
	}
}

func (d *DefaultContext) EventAllowed(events.Envelope_EventType) bool {
	return true
}

func (d *DefaultContext) Destination() string {
	return d.destination
}

func (d *DefaultContext) Origin() string {
	return d.origin
}

func (d *DefaultContext) AppID(envelope *events.Envelope) string {
	return envelope_extensions.GetAppId(envelope)
}

type LogAllowedContext struct {
	DefaultContext
}

func NewLogAllowedContext(origin string, destination string) *LogAllowedContext {
	return &LogAllowedContext{
		DefaultContext{
			destination: destination,
			origin:      origin,
		},
	}
}

func (l *LogAllowedContext) EventAllowed(event events.Envelope_EventType) bool {
	return event == events.Envelope_LogMessage
}

type SystemContext struct {
	DefaultContext
}

func NewSystemContext(origin string, destination string) *SystemContext {
	return &SystemContext{
		DefaultContext{
			destination: destination,
			origin:      origin,
		},
	}
}

func (s *SystemContext) AppID(*events.Envelope) string {
	return envelope_extensions.SystemAppId
}
