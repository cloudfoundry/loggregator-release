package listener

import (
	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/gorilla/websocket"
	"io"
	"sync"
	"time"
)

type websocketListener struct {
	sync.WaitGroup
}

func NewWebsocket() *websocketListener {
	return new(websocketListener)
}

func (l *websocketListener) Start(url, appId string, outputChan OutputChannel, stopChan StopChannel) error {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return err
	}

	serverError := make(chan struct{})
	l.Add(1)
	go func() {
		defer l.Done()
		select {
		case <-stopChan:
			conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Time{})
		case <-serverError:
		}
	}()

	for {
		_, msg, err := conn.ReadMessage()

		if err == io.EOF {
			close(serverError)
			break
		}

		if err != nil {
			outputChan <- generateLogMessage("proxy: error connecting to a loggregator server", appId)
			close(serverError)
			break
		}
		outputChan <- msg
	}

	l.Wait()
	return nil
}

func generateLogMessage(messageString string, appId string) []byte {
	messageType := logmessage.LogMessage_ERR
	currentTime := time.Now()
	logMessage := &logmessage.LogMessage{
		Message:     []byte(messageString),
		AppId:       proto.String(appId),
		MessageType: &messageType,
		SourceName:  proto.String("LGR"),
		Timestamp:   proto.Int64(currentTime.UnixNano()),
	}

	msg, _ := proto.Marshal(logMessage)
	return msg
}
