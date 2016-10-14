package doppler_test

import (
	"sync/atomic"

	"github.com/cloudfoundry/sonde-go/events"
)

type fakeWriter struct {
	openFileDescriptors uint64
	lastUptime          uint64
}

func (f *fakeWriter) Write(message *events.Envelope) {
	if message.GetEventType() == events.Envelope_ValueMetric {
		switch message.GetValueMetric().GetName() {
		case "Uptime":
			atomic.StoreUint64(&f.lastUptime, uint64(message.GetValueMetric().GetValue()))
		case "LinuxFileDescriptor":
			atomic.StoreUint64(&f.openFileDescriptors, uint64(message.GetValueMetric().GetValue()))
		}
	}
}
