package websocketserver_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/gorilla/websocket"
	"net/http"
	"testing"
	"time"
)

var logger = loggertesthelper.Logger()

func TestWebsocketServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "WebsocketServer Suite")
}

func NewMessageWithError(messageString, appId string) (*logmessage.Message, error) {
	logMessage := generateLogMessage(messageString, appId, logmessage.LogMessage_OUT, "App", "")

	marshalledLogMessage, err := proto.Marshal(logMessage)

	return logmessage.NewMessage(logMessage, marshalledLogMessage), err
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

func AddWSSink(receivedChan chan []byte, url string) (chan struct{}, <-chan struct{}) {
	stopKeepAlive := make(chan struct{})
	connectionDropped := make(chan struct{})

	ws, _, err := websocket.DefaultDialer.Dial(url, http.Header{})
	if err != nil {
		close(stopKeepAlive)
		close(connectionDropped)
		return stopKeepAlive, connectionDropped
	}

	ws.SetPingHandler(func(message string) error {
		select {
		case <-stopKeepAlive:
			return nil
		default:
			return ws.WriteControl(websocket.PongMessage, []byte(message), time.Time{})
		}
	})

	go func() {
		defer close(connectionDropped)
		for {
			_, data, err := ws.ReadMessage()
			if err != nil {
				return
			}

			receivedChan <- data
		}
	}()

	return stopKeepAlive, connectionDropped
}

func parseLogMessage(actual []byte) (*logmessage.LogMessage, error) {
	receivedMessage := &logmessage.LogMessage{}
	err := proto.Unmarshal(actual, receivedMessage)
	return receivedMessage, err
}
