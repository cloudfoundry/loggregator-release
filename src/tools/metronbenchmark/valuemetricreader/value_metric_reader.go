package valuemetricreader

import (
	"github.com/cloudfoundry/sonde-go/events"
)

const TestOrigin = "test-origin"

type counter interface {
	IncrementValue()
}

type MessageReader interface {
	Read() *events.Envelope
}

type ValueMetricReader struct {
	receivedCounter counter
	reader          MessageReader
}

func NewValueMetricReader(receivedCounter counter, reader MessageReader) *ValueMetricReader {
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
