package sinkserver_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/gorilla/websocket"
	"net/http"
	"testing"
	"time"
)

func NewMessage(messageString, appId string) *logmessage.Message {
	logMessage := generateLogMessage(messageString, appId, logmessage.LogMessage_OUT, "App", "")

	marshalledLogMessage, err := proto.Marshal(logMessage)
	if err != nil {
		Fail(err.Error())
	}

	return logmessage.NewMessage(logMessage, marshalledLogMessage)
}

func NewLogMessage(messageString, appId string) *logmessage.LogMessage {
	messageType := logmessage.LogMessage_OUT
	sourceName := "App"

	return generateLogMessage(messageString, appId, messageType, sourceName, "")
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

func TestSinkserver(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Sinkserver Suite")
}

func AddWSSink(receivedChan chan []byte, port string, path string) (*websocket.Conn, chan bool, <-chan bool) {

	dontKeepAliveChan := make(chan bool, 1)
	connectionDroppedChannel := make(chan bool, 1)
	ws, _, err := websocket.DefaultDialer.Dial("ws://localhost:"+port+path, http.Header{})

	if err != nil {
		Fail(err.Error())
	}
	return ws, dontKeepAliveChan, connectionDroppedChannel
}
