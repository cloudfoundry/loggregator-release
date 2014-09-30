package legacyproxy

import (
	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"net/http"
	"trafficcontroller/doppler_endpoint"
)

func NewLegacyHandlerProvider(dopplerHandlerProvider doppler_endpoint.HandlerProvider) doppler_endpoint.HandlerProvider {
	return func(messages <-chan []byte, logger *gosteno.Logger) http.Handler {

		legacyMessageChan := make(chan []byte)

		go func() {
			defer close(legacyMessageChan)
			for message := range messages {
				legacyMessage := translateMessage(message, logger)
				if legacyMessage != nil {
					legacyMessageChan <- legacyMessage
				}
			}
		}()

		return dopplerHandlerProvider(legacyMessageChan, logger)
	}
}

func translateMessage(message []byte, logger *gosteno.Logger) []byte {
	var receivedEnvelope events.Envelope
	err := proto.Unmarshal(message, &receivedEnvelope)
	if err != nil {
		logger.Errorf("Failed converting message from dropsonde to legacy: %v", err)
		return nil
	}

	logMessage := receivedEnvelope.GetLogMessage()
	if logMessage == nil {
		return nil
	}

	messageBytes, err := proto.Marshal(
		&logmessage.LogMessage{
			Message:     logMessage.GetMessage(),
			MessageType: (*logmessage.LogMessage_MessageType)(logMessage.MessageType),
			Timestamp:   proto.Int64(logMessage.GetTimestamp()),
			AppId:       proto.String(logMessage.GetAppId()),
			SourceId:    proto.String(logMessage.GetSourceInstance()),
			SourceName:  proto.String(logMessage.GetSourceType()),
		},
	)
	if err != nil {
		logger.Errorf("Failed marshalling converted dropsonde message: %v", err)
		return nil
	}

	return messageBytes
}
