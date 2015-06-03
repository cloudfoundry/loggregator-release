package marshaller

import (
	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	"time"
)

type MessageGenerator func(string, string) []byte

func LoggregatorLogMessage(messageString string, appId string) []byte {
	currentTime := time.Now()
	logMessage := &logmessage.LogMessage{
		Message:     []byte(messageString),
		AppId:       proto.String(appId),
		MessageType: logmessage.LogMessage_ERR.Enum(),
		SourceName:  proto.String("LGR"),
		Timestamp:   proto.Int64(currentTime.UnixNano()),
	}

	msg, _ := proto.Marshal(logMessage)
	return msg
}

func DropsondeLogMessage(messageString string, appId string) []byte {
	currentTime := time.Now()
	logMessage := &events.LogMessage{
		Message:     []byte(messageString),
		MessageType: events.LogMessage_ERR.Enum(),
		Timestamp:   proto.Int64(currentTime.UnixNano()),
		SourceType:  proto.String("DOP"),
		AppId:       &appId,
	}

	envelope, _ := emitter.Wrap(logMessage, "doppler")

	msg, _ := proto.Marshal(envelope)
	return msg
}
