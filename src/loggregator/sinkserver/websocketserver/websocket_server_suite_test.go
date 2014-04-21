package websocketserver_test

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

func AddWSSink(receivedChan chan []byte, url string) (*websocket.Conn, chan<- struct{}, <-chan struct{}) {
	stopKeepAlive := make(chan struct{})
	connectionDropped := make(chan struct{})

	ws, _, err := websocket.DefaultDialer.Dial(url, http.Header{})
	if err != nil {
		close(stopKeepAlive)
		close(connectionDropped)
		return nil, stopKeepAlive, connectionDropped
	}

	go func() {
		for {

			_, data, err := ws.ReadMessage()

			if err != nil {
				close(connectionDropped)
				close(receivedChan)
				return
			}
			receivedChan <- data
		}

	}()

	go func() {
		for {
			err := ws.WriteMessage(websocket.BinaryMessage, []byte{42})
			if err != nil {
				break
			}
			select {
			case <-stopKeepAlive:
				return
			case <-time.After(4 * time.Millisecond):
				// keep-alive
			}
		}
	}()
	return ws, stopKeepAlive, connectionDropped
}

func parseLogMessage(actual []byte) (*logmessage.LogMessage, error) {
	receivedMessage := &logmessage.LogMessage{}
	err := proto.Unmarshal(actual, receivedMessage)
	return receivedMessage, err
}
