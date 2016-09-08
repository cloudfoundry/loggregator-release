package eventmarshaller

import (
	"sync"

	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

//go:generate hel --type BatchChainByteWriter --output mock_writer_test.go

// BatchChainByteWriter is a byte writer than can accept a series
// of metricbatcher.BatchCounterChainer values.  It should add any
// additional tags it needs and send the chainer when the message
// is successfully sent.
type BatchChainByteWriter interface {
	Write(message []byte, chainers ...metricbatcher.BatchCounterChainer) (sentLength int, err error)
}

//go:generate hel --type EventBatcher --output mock_event_batcher_test.go

type EventBatcher interface {
	BatchCounter(name string) (chainer metricbatcher.BatchCounterChainer)
	BatchIncrementCounter(name string)
}

type EventMarshaller struct {
	batcher    EventBatcher
	logger     *gosteno.Logger
	byteWriter BatchChainByteWriter
	bwLock     sync.RWMutex
}

func New(batcher EventBatcher, logger *gosteno.Logger) *EventMarshaller {
	return &EventMarshaller{
		batcher: batcher,
		logger:  logger,
	}
}

func (m *EventMarshaller) SetWriter(byteWriter BatchChainByteWriter) {
	m.bwLock.Lock()
	defer m.bwLock.Unlock()
	m.byteWriter = byteWriter
}

func (m *EventMarshaller) writer() BatchChainByteWriter {
	m.bwLock.RLock()
	defer m.bwLock.RUnlock()
	return m.byteWriter
}

func (m *EventMarshaller) Write(envelope *events.Envelope) {
	writer := m.writer()
	if writer == nil {
		m.logger.Warn("EventMarshaller: Write called while byteWriter is nil")
		return
	}

	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		m.logger.Errorf("marshalling error: %v", err)
		m.batcher.BatchIncrementCounter("dropsondeMarshaller.marshalErrors")
		return
	}

	chainer := m.batcher.BatchCounter("dropsondeMarshaller.sentEnvelopes").
		SetTag("event_type", envelope.GetEventType().String())

	writer.Write(envelopeBytes, chainer)
}
