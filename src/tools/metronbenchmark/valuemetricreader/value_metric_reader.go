package valuemetricreader

import (
	"github.com/cloudfoundry/sonde-go/events"
)

const TestOrigin = "test-origin"

type MetricsReporter interface {
	IncrementReceivedMessages()
}

type MessageReader interface {
	Read() *events.Envelope
}

type ValueMetricReader struct {
	reporter MetricsReporter
	reader   MessageReader
}

func NewValueMetricReader(reporter MetricsReporter, reader MessageReader) *ValueMetricReader {
	return &ValueMetricReader{
		reporter: reporter,
		reader:   reader,
	}
}

func (lr *ValueMetricReader) Read() {
	message := lr.reader.Read()

	if message != nil && message.GetValueMetric() != nil && *message.Origin == TestOrigin {
		lr.reporter.IncrementReceivedMessages()
	}
}
