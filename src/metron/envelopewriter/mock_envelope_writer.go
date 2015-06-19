package envelopewriter

import "github.com/cloudfoundry/sonde-go/events"

type MockEnvelopeWriter struct {
	Events []*events.Envelope
}

func (m *MockEnvelopeWriter) Write(event *events.Envelope) error {
	m.Events = append(m.Events, event)
	return nil
}
