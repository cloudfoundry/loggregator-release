package legacy_unmarshaller

import (
	"sync/atomic"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/davecgh/go-spew/spew"
	"github.com/gogo/protobuf/proto"
)

type LegacyUnmarshaller struct {
	logger              *gosteno.Logger
	unmarshalErrorCount uint64
}

func New(logger *gosteno.Logger) *LegacyUnmarshaller {
	return &LegacyUnmarshaller{
		logger: logger,
	}
}

func (u *LegacyUnmarshaller) Run(inputChan <-chan []byte, outputChan chan<- *logmessage.LogEnvelope) {
	for message := range inputChan {
		envelope, err := u.UnmarshalMessage(message)
		if err != nil {
			continue
		}
		outputChan <- envelope
	}
}

func (u *LegacyUnmarshaller) UnmarshalMessage(message []byte) (*logmessage.LogEnvelope, error) {
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

func (m *LegacyUnmarshaller) Emit() instrumentation.Context {
	return instrumentation.Context{
		Name:    "legacyUnmarshaller",
		Metrics: m.metrics(),
	}
}

func incrementCount(count *uint64) {
	atomic.AddUint64(count, 1)
}

func (m *LegacyUnmarshaller) metrics() []instrumentation.Metric {
	var metrics []instrumentation.Metric

	metrics = append(metrics, instrumentation.Metric{
		Name:  "unmarshalErrors",
		Value: atomic.LoadUint64(&m.unmarshalErrorCount),
	})

	return metrics
}
