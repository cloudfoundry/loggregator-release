package legacyreader

import (
	"github.com/cloudfoundry/sonde-go/events"
	"tools/benchmark/metricsreporter"
	"tools/metronbenchmark/valuemetricreader"
)

const legacyOrigin = "legacy"

type MessageReader interface {
	Read() *events.Envelope
}

type LegacyReader struct {
	receivedCounter        *metricsreporter.Counter
	internalMetricsCounter *metricsreporter.Counter
	reader                 MessageReader
}

func NewLegacyReader(receivedCounter *metricsreporter.Counter, internalMetricsCounter *metricsreporter.Counter, reader MessageReader) *LegacyReader {
	return &LegacyReader{
		receivedCounter:        receivedCounter,
		internalMetricsCounter: internalMetricsCounter,
		reader:                 reader,
	}
}

func (lr *LegacyReader) Read() {
	message := lr.reader.Read()

	if message != nil {
		if *message.Origin == valuemetricreader.TestOrigin || *message.Origin == legacyOrigin {
			lr.receivedCounter.IncrementValue()
		} else {
			lr.internalMetricsCounter.IncrementValue()
		}
	}
}
