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
			conn.Close()
		case <-serverError:
		}
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			errorMsg := fmt.Sprintf("proxy: error connecting to a loggregator server")
			outputChan <- generateLogMessage(errorMsg, appId)
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
