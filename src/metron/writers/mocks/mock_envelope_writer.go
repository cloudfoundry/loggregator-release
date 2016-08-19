package mocks

import (
	"sync"

	"github.com/cloudfoundry/sonde-go/events"
)

type MockEnvelopeWriter struct {
	Events     []*events.Envelope
	eventsLock sync.Mutex
}

func (m *MockEnvelopeWriter) Write(event *events.Envelope) {
	m.eventsLock.Lock()
	defer m.eventsLock.Unlock()
	m.Events = append(m.Events, event)
}
