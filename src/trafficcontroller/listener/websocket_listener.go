package listener

import (
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/gorilla/websocket"
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
		conn.SetReadDeadline(deadline(l.timeout))
		_, msg, err := conn.ReadMessage()

		// Error message from Gorilla Websocket library is not a constant/variable, so we only can check the contents of the message.
		if err != nil && err.Error() == "websocket: close 1005 " {
			close(serverError)
			break
		}

		if err != nil {
			isTimeout, _ := regexp.MatchString(`i/o timeout`, err.Error())
			if isTimeout {
				descriptiveError := fmt.Errorf("WebsocketListener.Start: Timed out listening to %s after %s", url, l.timeout.String())
				outputChan <- l.generateLogMessage(descriptiveError.Error(), appId)
				close(serverError)
				l.Wait()
				return descriptiveError
			}

			outputChan <- l.generateLogMessage("WebsocketListener.Start: Error connecting to a loggregator/doppler server", appId)
			close(serverError)
			break
		}
		convertedMessage, err := l.convertLogMessage(msg)
		if err != nil {
			l.logger.Errorf("WebsocketListener.Start: Error converting message %v. Message: %v", msg, err)
		} else {
			outputChan <- convertedMessage
		}
	}

	l.Wait()
	return nil
}

func deadline(timeout time.Duration) time.Time {
	if timeout == 0 {
		return time.Time{}
	}

	return time.Now().Add(timeout)
}
