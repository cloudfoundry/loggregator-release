package logmessagereader

import (
	"github.com/cloudfoundry/sonde-go/events"
)

type MetricsReporter interface {
	IncrementReceivedMessages()
}

type MessageReader interface {
	Read() *events.Envelope
}

type LogMessageReader struct {
	reporter MetricsReporter
	reader   MessageReader
}

func NewLogMessageReader(reporter MetricsReporter, reader MessageReader) *LogMessageReader {
	return &LogMessageReader{
		reporter: reporter,
		reader:   reader,
	}
}

func (lr *LogMessageReader) Read() {
	message := lr.reader.Read()

	if message != nil && message.GetLogMessage() != nil {
		lr.reporter.IncrementReceivedMessages()
	}
}
