package legacyreader

import (
	"github.com/cloudfoundry/sonde-go/events"
	"tools/metronbenchmark/valuemetricreader"
)

const legacyOrigin = "legacy"

type MetricsReporter interface {
	IncrementReceivedMessages()
}

type MessageReader interface {
	Read() *events.Envelope
}

type LegacyReader struct {
	reporter MetricsReporter
	reader   MessageReader
}

func NewLegacyReader(reporter MetricsReporter, reader MessageReader) *LegacyReader {
	return &LegacyReader{
		reporter: reporter,
		reader:   reader,
	}
}

func (lr *LegacyReader) Read() {
	message := lr.reader.Read()

	if message != nil && (*message.Origin == valuemetricreader.TestOrigin || *message.Origin == legacyOrigin) {
		lr.reporter.IncrementReceivedMessages()
	}
}
