package websocket_test

import "github.com/cloudfoundry/sonde-go/events"

type fakeCounter struct {
	incrementCalls chan events.Envelope_EventType
}

func (f *fakeCounter) Increment(typ events.Envelope_EventType) {
	f.incrementCalls <- typ
}
