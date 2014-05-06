package listener

import (
	"code.google.com/p/gogoprotobuf/proto"
	"fmt"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/gorilla/websocket"
	"sync"
	"time"
)

type websocketListener struct {
	sync.WaitGroup
	appId string
}

func NewWebsocket(appId string) *websocketListener {
	return &websocketListener{appId: appId}
}

func (l *websocketListener) Start(url string, outputChan OutputChannel, stopChan StopChannel) error {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return err
	}

	serverError := make(chan struct{})
	l.Add(2)
	go func() {
		defer l.Done()
		select {
		case <-stopChan:
			conn.Close()
		case <-serverError:
		}
	}()

	go func() {
		defer l.Done()
		defer close(serverError)

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				errorMsg := fmt.Sprintf("proxy: error connecting to a loggregator server")
				outputChan <- generateLogMessage(errorMsg, l.appId)
				return
			}
			outputChan <- msg
		}
	}()

	return nil
}

func (l *websocketListener) Wait() {
	l.WaitGroup.Wait()
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
