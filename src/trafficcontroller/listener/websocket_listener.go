package listener

import (
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/gorilla/websocket"
	"io"
	"regexp"
	"sync"
	"time"
	"trafficcontroller/marshaller"
)

type websocketListener struct {
	sync.WaitGroup
	generateLogMessage marshaller.MessageGenerator
	convertLogMessage  MessageConverter
	timeout            time.Duration
	logger             *gosteno.Logger
}

type MessageConverter func([]byte) ([]byte, error)

func NewWebsocket(logMessageGenerator marshaller.MessageGenerator, messageConverter MessageConverter, timeout time.Duration, logger *gosteno.Logger) *websocketListener {
	return &websocketListener{
		generateLogMessage: logMessageGenerator,
		convertLogMessage:  messageConverter,
		timeout:            timeout,
		logger:             logger,
	}
}

func (l *websocketListener) Start(url, appId string, outputChan OutputChannel, stopChan StopChannel) error {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return err
	}

loop:
	for {
		select {
		case <-stopChan:
			conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Time{})
			return nil

		default:
			conn.SetReadDeadline(deadline(l.timeout))
			_, msg, err := conn.ReadMessage()

			if err == io.EOF {
				break loop
			}

			if err != nil {
				isTimeout, _ := regexp.MatchString(`i/o timeout`, err.Error())
				if isTimeout {
					descriptiveError := fmt.Errorf("WebsocketListener.Start: Timed out listening to %s after %s", url, l.timeout.String())
					outputChan <- l.generateLogMessage(descriptiveError.Error(), appId)
					return descriptiveError
				}

				outputChan <- l.generateLogMessage("WebsocketListener.Start: Error connecting to a loggregator/doppler server", appId)
				break loop
			}

			convertedMessage, err := l.convertLogMessage(msg)
			if err != nil {
				l.logger.Errorf("WebsocketListener.Start: Error converting message %v. Message: %v", msg, err)
			} else {
				outputChan <- convertedMessage
			}
		}
	}

	return nil
}

func deadline(timeout time.Duration) time.Time {
	if timeout == 0 {
		return time.Time{}
	}

	return time.Now().Add(timeout)
}
