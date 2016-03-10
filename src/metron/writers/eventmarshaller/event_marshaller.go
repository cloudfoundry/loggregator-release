package eventmarshaller

import (
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
}

func New(logger *gosteno.Logger, byteWriter ByteWriter) *EventMarshaller {

	return &EventMarshaller{
		logger:     logger,
		byteWriter: byteWriter,
	}
}

func (p *EventMarshaller) Write(envelope *events.Envelope) {

	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		p.logger.Errorf("marshalling error: %v", err)
		metrics.BatchIncrementCounter("dropsondeMarshaller.marshalErrors")
		return
	}

	p.byteWriter.Write(envelopeBytes)
}
