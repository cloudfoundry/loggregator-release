package truncatingbuffer

import "github.com/cloudfoundry/sonde-go/events"

type BufferContext interface {
	EventAllowed(events.Envelope_EventType) bool
	Identifier() string
	DropsondeOrigin() string
}

type DefaultContext struct {
	identifier string
	dropsondeOrigin string
}

func NewDefaultContext(origin string, identifier string) *DefaultContext{
	return &DefaultContext {
		identifier: identifier,
		dropsondeOrigin: origin,
	}
}

func(d *DefaultContext) EventAllowed(events.Envelope_EventType) bool{
	return true
}

func(d *DefaultContext) Identifier() string {
	return d.identifier
}

func (d *DefaultContext) DropsondeOrigin() string {
	return d.dropsondeOrigin
}

type LogAllowedContext struct {
	DefaultContext
}

func NewLogAllowedContext(origin string, identifier string) *LogAllowedContext {
	return &LogAllowedContext{
		DefaultContext {
			identifier: identifier,
			dropsondeOrigin: origin,
		},
	}
}

func (l *LogAllowedContext) EventAllowed(event events.Envelope_EventType) bool {
	return event == events.Envelope_LogMessage
}