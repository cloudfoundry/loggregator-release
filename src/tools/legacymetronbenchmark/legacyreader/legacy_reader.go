package legacyreader

import (
	"github.com/cloudfoundry/sonde-go/events"
	"tools/metronbenchmark/valuemetricreader"
)

const legacyOrigin = "legacy"

type counter interface {
	IncrementValue()
}

type MessageReader interface {
	Read() *events.Envelope
}

type LegacyReader struct {
	receivedCounter counter
	reader          MessageReader
}

func NewLegacyReader(receivedCounter counter, reader MessageReader) *LegacyReader {
	return &LegacyReader{
		receivedCounter: receivedCounter,
		reader:          reader,
	}
}

func (lr *LegacyReader) Read() {
	message := lr.reader.Read()

	if message != nil && (*message.Origin == valuemetricreader.TestOrigin || *message.Origin == legacyOrigin) {
		lr.receivedCounter.IncrementValue()
	}
}
