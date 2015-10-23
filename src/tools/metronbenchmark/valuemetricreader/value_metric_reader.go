package valuemetricreader

import (
	"tools/benchmark/metricsreporter"

	"github.com/cloudfoundry/sonde-go/events"
)

const TestOrigin = "test-origin"

type MessageReader interface {
	Read() *events.Envelope
	Close()
}

type ValueMetricReader struct {
	receivedCounter *metricsreporter.Counter
	reader          MessageReader
}

func NewValueMetricReader(receivedCounter *metricsreporter.Counter, reader MessageReader) *ValueMetricReader {
	return &ValueMetricReader{
		receivedCounter: receivedCounter,
		reader:          reader,
	}
}

func (lr *ValueMetricReader) Read() {
	message := lr.reader.Read()

	if message != nil && message.GetValueMetric() != nil && *message.Origin == TestOrigin {
		lr.receivedCounter.IncrementValue()
	}
}

func (lr *ValueMetricReader) Close() {
	lr.reader.Close()
}
