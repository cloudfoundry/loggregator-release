package envelopewrapper

import (
	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/davecgh/go-spew/spew"
	"sync/atomic"
)

type WrappedEnvelope struct {
	Envelope      *events.Envelope
	EnvelopeBytes []byte
}

func (we *WrappedEnvelope) EnvelopeLength() int {
	return len(we.EnvelopeBytes)
}

func WrapEvent(event events.Event, origin string) (*WrappedEnvelope, error) {
	envelope, err := emitter.Wrap(event, origin)

	if err != nil {
		return nil, err
	}

	envelopeBytes, err := proto.Marshal(envelope)

	if err != nil {
		return nil, err
	}

	wrappedEnv := &WrappedEnvelope{
		Envelope:      envelope,
		EnvelopeBytes: envelopeBytes,
	}

	return wrappedEnv, nil
}

type EnvelopeWrapper interface {
	instrumentation.Instrumentable
	Run(inputChan <-chan *events.Envelope, outputChan chan<- *WrappedEnvelope)
}

func NewEnvelopeWrapper(logger *gosteno.Logger) EnvelopeWrapper {
	return &envelopeWrapper{
		logger: logger,
	}
}

type envelopeWrapper struct {
	logger            *gosteno.Logger
	marshalErrorCount uint64
}

func (u *envelopeWrapper) Run(inputChan <-chan *events.Envelope, outputChan chan<- *WrappedEnvelope) {
	for envelope := range inputChan {

		messageBytes, err := proto.Marshal(envelope)
		if err != nil {
			u.logger.Errorf("envelopeWrapper: marshal error %v for message %v", err, envelope)
			incrementCount(&u.marshalErrorCount)
			continue
		}

		u.logger.Debugf("envelopeWrapper: marshalled message %v", spew.Sprintf("%v", envelope))

		outputChan <- &WrappedEnvelope{Envelope: envelope, EnvelopeBytes: messageBytes}
	}
}

func incrementCount(count *uint64) {
	atomic.AddUint64(count, 1)
}

func (m *envelopeWrapper) metrics() []instrumentation.Metric {
	var metrics []instrumentation.Metric

	metrics = append(metrics, instrumentation.Metric{
		Name:  "marshalErrors",
		Value: atomic.LoadUint64(&m.marshalErrorCount),
	})

	return metrics
}

// Emit returns the current metrics the DropsondeMarshaller keeps about itself.
func (m *envelopeWrapper) Emit() instrumentation.Context {
	return instrumentation.Context{
		Name:    "envelopeWrapper",
		Metrics: m.metrics(),
	}
}
