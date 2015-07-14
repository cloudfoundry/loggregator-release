package synchronizedwriter

import (
	"github.com/cloudfoundry/sonde-go/events"
	"metron/writers"
	"sync"
)

type SynchronizedWriter struct {
	writer writers.EnvelopeWriter
	lock   sync.Mutex
}

func New(writer writers.EnvelopeWriter) *SynchronizedWriter {
	return &SynchronizedWriter{
		writer: writer,
	}
}

func (sw *SynchronizedWriter) Write(envelope *events.Envelope) {
	sw.lock.Lock()
	defer sw.lock.Unlock()
	sw.writer.Write(envelope)
}
