package sinkmanager_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"time"
)

func TestSinkmanager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sinkmanager Suite")
}

func NewMessage(messageString, appId string) *logmessage.Message {
	logMessage := generateLogMessage(messageString, appId, logmessage.LogMessage_OUT, "App", "")

	marshalledLogMessage, err := proto.Marshal(logMessage)
	if err != nil {
		Fail(err.Error())
	}

	return logmessage.NewMessage(logMessage, marshalledLogMessage)
}

func generateLogMessage(messageString, appId string, messageType logmessage.LogMessage_MessageType, sourceName, sourceId string) *logmessage.LogMessage {
	currentTime := time.Now()
	logMessage := &logmessage.LogMessage{
Message:     []byte(messageString),
AppId:       proto.String(appId),
	MessageType: &messageType,
	SourceName:  proto.String(sourceName),
	SourceId:    proto.String(sourceId),
	Timestamp:   proto.Int64(currentTime.UnixNano()),
}

return logMessage
}
