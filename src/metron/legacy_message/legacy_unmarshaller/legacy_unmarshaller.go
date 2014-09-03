package legacy_unmarshaller

import (
	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/davecgh/go-spew/spew"
	"sync/atomic"
)

type LegacyUnmarshaller interface {
	instrumentation.Instrumentable
	Run(inputChan <-chan []byte, outputChan chan<- *logmessage.LogEnvelope)
	UnmarshalMessage([]byte) (*logmessage.LogEnvelope, error)
}

func NewLegacyUnmarshaller(logger *gosteno.Logger) LegacyUnmarshaller {
	return &legacyUnmarshaller{
		logger: logger,
	}
}

type legacyUnmarshaller struct {
	logger              *gosteno.Logger
	unmarshalErrorCount uint64
}

func (u *legacyUnmarshaller) Run(inputChan <-chan []byte, outputChan chan<- *logmessage.LogEnvelope) {
	for message := range inputChan {
		envelope, err := u.UnmarshalMessage(message)
		if err != nil {
			continue
		}
		outputChan <- envelope
	}
}

func (u *legacyUnmarshaller) UnmarshalMessage(message []byte) (*logmessage.LogEnvelope, error) {
	envelope := &logmessage.LogEnvelope{}
	err := proto.Unmarshal(message, envelope)
	if err != nil {
		u.logger.Debugf("legacyUnmarshaller: unmarshal error %v for message %v", err, message)
		incrementCount(&u.unmarshalErrorCount)
		return nil, err
	}

	u.logger.Debugf("legacyUnmarshaller: received message %v", spew.Sprintf("%v", envelope))

	return envelope, nil
}

func incrementCount(count *uint64) {
	atomic.AddUint64(count, 1)
}

func (m *legacyUnmarshaller) metrics() []instrumentation.Metric {
	var metrics []instrumentation.Metric

	metrics = append(metrics, instrumentation.Metric{
		Name:  "unmarshalErrors",
		Value: atomic.LoadUint64(&m.unmarshalErrorCount),
	})

	return metrics
}

func (m *legacyUnmarshaller) Emit() instrumentation.Context {
	return instrumentation.Context{
		Name:    "legacyUnmarshaller",
		Metrics: m.metrics(),
	}
}
