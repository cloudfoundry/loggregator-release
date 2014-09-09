package listener

import (
	"github.com/gorilla/websocket"
	"io"
	"sync"
	"time"
	"trafficcontroller/marshaller"
)

type websocketListener struct {
	sync.WaitGroup
	generateLogMessage marshaller.MessageGenerator
}

func NewWebsocket(logMessageGenerator marshaller.MessageGenerator) *websocketListener {
	return &websocketListener{generateLogMessage: logMessageGenerator}
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
			outputChan <- l.generateLogMessage("proxy: error connecting to a loggregator server", appId)
			close(serverError)
			break
		}
		outputChan <- msg
	}

	l.Wait()
	return nil
}
