package legacy_message_converter

import (
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/davecgh/go-spew/spew"
	"github.com/gogo/protobuf/proto"
)

const LEGACY_DROPSONDE_ORIGIN = "legacy"

func New(logger *gosteno.Logger) *LegacyMessageConverter {
	return &LegacyMessageConverter{
		logger: logger,
	}
}

type LegacyMessageConverter struct {
	logger *gosteno.Logger
}

func (c *LegacyMessageConverter) Run(inputChan <-chan *logmessage.LogEnvelope, outputChan chan<- *events.Envelope) {
	for legacyEnvelope := range inputChan {
		c.logger.Debugf("legacyMessageConverter: converting message %v", spew.Sprintf("%v", legacyEnvelope))

		outputChan <- convertMessage(legacyEnvelope)
	}
}

func convertMessage(legacyEnvelope *logmessage.LogEnvelope) *events.Envelope {
	legacyMessage := legacyEnvelope.GetLogMessage()
	return &events.Envelope{
		Origin:    proto.String(LEGACY_DROPSONDE_ORIGIN),
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
