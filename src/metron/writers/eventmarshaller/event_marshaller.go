package eventmarshaller

import (
	"sync"

	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

//go:generate hel --type ByteWriter --output mock_writer_test.go

type ByteWriter interface {
	Write(message []byte) (sentLength int, err error)
}

type EventMarshaller struct {
	logger     *gosteno.Logger
	byteWriter ByteWriter
	bwLock     sync.RWMutex
}

func New(logger *gosteno.Logger) *EventMarshaller {
	return &EventMarshaller{
		logger: logger,
	}
}

func (m *EventMarshaller) SetWriter(byteWriter ByteWriter) {
	m.bwLock.Lock()
	defer m.bwLock.Unlock()
	m.byteWriter = byteWriter
}

func (m *EventMarshaller) writer() ByteWriter {
	m.bwLock.RLock()
	defer m.bwLock.RUnlock()
	return m.byteWriter
}

func (m *EventMarshaller) Write(envelope *events.Envelope) {
	writer := m.writer()
	if writer == nil {
		m.logger.Warn("EventMarshaller: Write called while byteWriter is nil")
		metrics.BatchIncrementCounter("dropsondeMarshaller.nilByteWriterWrites")
		return
	}

	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		m.logger.Errorf("marshalling error: %v", err)
		metrics.BatchIncrementCounter("dropsondeMarshaller.marshalErrors")
		return
	}

	writer.Write(envelopeBytes)
}
