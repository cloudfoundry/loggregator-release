package legacyunmarshaller

import (
	"metron/writers"

	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/davecgh/go-spew/spew"
	"github.com/gogo/protobuf/proto"
)

const legacyDropsondeOrigin = "legacy"

type LegacyUnmarshaller struct {
	outputWriter writers.EnvelopeWriter
	logger       *gosteno.Logger
}

func New(outputWriter writers.EnvelopeWriter, logger *gosteno.Logger) *LegacyUnmarshaller {
	return &LegacyUnmarshaller{
		outputWriter: outputWriter,
		logger:       logger,
	}
}

func (u *LegacyUnmarshaller) Write(message []byte) {
	legacyEnvelope, err := u.unmarshalMessage(message)
	if err != nil {
		return
	}

	dropsondeEnvelope := convertMessage(legacyEnvelope)
	u.outputWriter.Write(dropsondeEnvelope)
}

func (u *LegacyUnmarshaller) unmarshalMessage(message []byte) (*logmessage.LogEnvelope, error) {
	envelope := &logmessage.LogEnvelope{}
	err := proto.Unmarshal(message, envelope)
	if err != nil {
		u.logger.Debugf("legacyUnmarshaller: unmarshal error %v for message %v", err, message)
		metrics.BatchIncrementCounter("legacyUnmarshaller.unmarshalErrors")
		return nil, err
	}

	u.logger.Debugf("legacyUnmarshaller: received message %v", spew.Sprintf("%v", envelope))

	return envelope, nil
}

func convertMessage(legacyEnvelope *logmessage.LogEnvelope) *events.Envelope {
	legacyMessage := legacyEnvelope.GetLogMessage()
	return &events.Envelope{
		Origin:    proto.String(legacyDropsondeOrigin),
		EventType: events.Envelope_LogMessage.Enum(),
		LogMessage: &events.LogMessage{
			Message:        legacyMessage.Message,
			MessageType:    (*events.LogMessage_MessageType)(legacyMessage.MessageType),
			Timestamp:      legacyMessage.Timestamp,
			AppId:          legacyMessage.AppId,
			SourceType:     legacyMessage.SourceName,
			SourceInstance: legacyMessage.SourceId,
		},
	}
}
