package valuemetricreader

import (
	"github.com/cloudfoundry/sonde-go/events"
	"tools/benchmark/metricsreporter"
)

const TestOrigin = "test-origin"

type MessageReader interface {
	Read() *events.Envelope
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
