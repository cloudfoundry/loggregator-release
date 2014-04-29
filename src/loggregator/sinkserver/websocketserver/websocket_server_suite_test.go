package websocketserver_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/gorilla/websocket"
	"io"
	"net/http"
	"sync"
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

func AddWSSink(receivedChan chan []byte, url string) (*websocket.Conn, chan<- struct{}, <-chan struct{}) {
	stopKeepAlive := make(chan struct{})
	connectionDropped := make(chan struct{})

	lock := sync.RWMutex{}

	ws, _, err := websocket.DefaultDialer.Dial(url, http.Header{})
	if err != nil {
		close(stopKeepAlive)
		close(connectionDropped)
		return nil, stopKeepAlive, connectionDropped
	}

	go func() {
		lock.Lock()
		ws.SetPingHandler(func(message string) error {
			logger.Debugf("Client: got ping '%s' sending pong at %v\n", message, time.Now())
			return ws.WriteControl(websocket.PongMessage, []byte(message), time.Time{})
		})
		lock.Unlock()

		select {
		case <-stopKeepAlive:
			logger.Debugf("Client: asked to stop keepalive")
			lock.Lock()
			ws.SetPingHandler(func(string) error { logger.Debugf("Client: ignoring ping"); return nil })
			lock.Unlock()
			close(connectionDropped)
		case <-connectionDropped:
			logger.Debugf("Client: connection closed stopping keepalive")
		}
	}()

	go func() {
		for {
			lock.RLock()
			_, data, err := ws.ReadMessage()
			lock.RUnlock()

			if err == io.EOF {
				continue
			}
			if err != nil {
				logger.Debugf("Client: error %s reading from websocket, closing connection\n", err)
				close(connectionDropped)
				return
			}

			receivedChan <- data
		}

	}()

	return ws, stopKeepAlive, connectionDropped
}

func parseLogMessage(actual []byte) (*logmessage.LogMessage, error) {
	receivedMessage := &logmessage.LogMessage{}
	err := proto.Unmarshal(actual, receivedMessage)
	return receivedMessage, err
}
