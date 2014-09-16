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
	convertLogMessage  MessageConverter
}

type MessageConverter func([]byte) []byte

func NewWebsocket(logMessageGenerator marshaller.MessageGenerator, messageConverter MessageConverter) *websocketListener {
	return &websocketListener{generateLogMessage: logMessageGenerator, convertLogMessage: messageConverter}
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
			outputChan <- l.generateLogMessage("proxy: error connecting to a loggregator/doppler server", appId)
			close(serverError)
			break
		}
		outputChan <- l.convertLogMessage(msg)
	}

	l.Wait()
	return nil
}
