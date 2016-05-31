package listener

import (
	"fmt"
	"regexp"
	"sync"
	"time"
	"trafficcontroller/marshaller"

	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/gosteno"
	"github.com/gorilla/websocket"
)

//go:generate hel --type Batcher --output mock_batcher_test.go

type Batcher interface {
	BatchCounter(name string) metricbatcher.BatchCounterChainer
}

type WebsocketListener struct {
	sync.WaitGroup
	generateLogMessage marshaller.MessageGenerator
	convertLogMessage  MessageConverter
	readTimeout        time.Duration
	handshakeTimeout   time.Duration
	batcher            Batcher
	logger             *gosteno.Logger
}

type MessageConverter func([]byte) ([]byte, error)

func NewWebsocket(
	logMessageGenerator marshaller.MessageGenerator,
	messageConverter MessageConverter,
	readTimeout time.Duration,
	handshakeTimeout time.Duration,
	batcher Batcher,
	logger *gosteno.Logger,
) *WebsocketListener {
	return &WebsocketListener{
		generateLogMessage: logMessageGenerator,
		convertLogMessage:  messageConverter,
		readTimeout:        readTimeout,
		handshakeTimeout:   handshakeTimeout,
		batcher:            batcher,
		logger:             logger,
	}
}

func (l *WebsocketListener) Start(url string, appId string, outputChan OutputChannel, stopChan StopChannel) error {
	dialer := &websocket.Dialer{
		HandshakeTimeout: l.handshakeTimeout,
	}
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return err
	}

	go func() {
		<-stopChan
		conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Time{})
		conn.Close()
	}()

	return l.listenWithTimeout(l.readTimeout, url, appId, conn, outputChan)
}

func (l *WebsocketListener) listenWithTimeout(readTimeout time.Duration, url string, appId string, conn *websocket.Conn, outputChan OutputChannel) error {
	for {
		conn.SetReadDeadline(deadline(readTimeout))
		_, msg, err := conn.ReadMessage()

		if wsErr, ok := err.(*websocket.CloseError); ok {
			if wsErr.Code == websocket.CloseNormalClosure {
				return nil
			}
		}

		if err != nil {
			isTimeout, _ := regexp.MatchString(`i/o timeout`, err.Error())
			if isTimeout {
				l.logger.Errorf("WebsocketListener.Start: Timed out listening to %s after %s", url, l.readTimeout.String())
				descriptiveError := fmt.Errorf("WebsocketListener.Start: Timed out listening to a doppler server after %s", l.readTimeout.String())
				outputChan <- l.generateLogMessage(descriptiveError.Error(), appId)
				return descriptiveError
			}

			isClosed, _ := regexp.MatchString(`use of closed network connection`, err.Error())
			if isClosed {
				return nil
			}

			l.logger.Errorf("WebsocketListener.Start: Error connecting to %s: %s", url, err.Error())
			outputChan <- l.generateLogMessage("WebsocketListener.Start: Error connecting to a doppler server", appId)
			return nil
		}

		convertedMessage, err := l.convertLogMessage(msg)
		if err != nil {
			l.logger.Errorf("WebsocketListener.Start: failed to convert log message %v", err)
			continue
		}
		if convertedMessage != nil {
			l.batcher.BatchCounter("listeners.receivedEnvelopes").
				SetTag("protocol", "ws").
				Increment()
			outputChan <- convertedMessage
		}
	}
}

func deadline(timeout time.Duration) time.Time {
	if timeout == 0 {
		return time.Time{}
	}

	return time.Now().Add(timeout)
}
