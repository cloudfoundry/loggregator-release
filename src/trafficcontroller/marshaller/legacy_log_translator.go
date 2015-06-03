package marshaller

import (
	"fmt"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

func TranslateDropsondeToLegacyLogMessage(message []byte) ([]byte, error) {
	var receivedEnvelope events.Envelope
	err := proto.Unmarshal(message, &receivedEnvelope)
	if err != nil {
		return nil, fmt.Errorf("TranslateDropsondeToLegacyLogMessage: Unable to unmarshal bytes as Envelope: %v", err)
	}

	if receivedEnvelope.GetEventType() != events.Envelope_LogMessage {
		return nil, fmt.Errorf("TranslateDropsondeToLegacyLogMessage: Envelope contained %s instead of LogMessage", receivedEnvelope.GetEventType().String())
	}

	logMessage := receivedEnvelope.GetLogMessage()
	if logMessage == nil {
		return nil, fmt.Errorf("TranslateDropsondeToLegacyLogMessage: Envelope's LogMessage was nil: %v", receivedEnvelope)
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
		return nil, fmt.Errorf("TranslateDropsondeToLegacyLogMessage: Failed marshalling converted dropsonde message: %v", err)
	}

	return messageBytes, nil
}
