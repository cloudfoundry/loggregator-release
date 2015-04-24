package legacymessage

import (
	"sync/atomic"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/davecgh/go-spew/spew"
	"github.com/gogo/protobuf/proto"
)

type Unmarshaller struct {
	logger              *gosteno.Logger
	unmarshalErrorCount uint64
}

func NewUnmarshaller(logger *gosteno.Logger) *Unmarshaller {
	return &Unmarshaller{
		logger: logger,
	}
}

func (u *Unmarshaller) Run(inputChan <-chan []byte, outputChan chan<- *logmessage.LogEnvelope) {
	for message := range inputChan {
		envelope, err := u.UnmarshalMessage(message)
		if err != nil {
			continue
		}
		outputChan <- envelope
	}
}

func (u *Unmarshaller) UnmarshalMessage(message []byte) (*logmessage.LogEnvelope, error) {
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

func (u *Unmarshaller) Emit() instrumentation.Context {
	return instrumentation.Context{
		Name:    "legacyUnmarshaller",
		Metrics: u.metrics(),
	}
}

func incrementCount(count *uint64) {
	atomic.AddUint64(count, 1)
}

func (u *Unmarshaller) metrics() []instrumentation.Metric {
	var metrics []instrumentation.Metric

	metrics = append(metrics, instrumentation.Metric{
		Name:  "unmarshalErrors",
		Value: atomic.LoadUint64(&u.unmarshalErrorCount),
	})

	return metrics
}
