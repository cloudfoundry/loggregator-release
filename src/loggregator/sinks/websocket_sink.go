package sinks

import (
	"code.google.com/p/go.net/websocket"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"net"
	"sync/atomic"
	"time"
)

type WebsocketSink struct {
	logger            *gosteno.Logger
	appId             string
	ws                *websocket.Conn
	clientAddress     net.Addr
	sentMessageCount  *uint64
	sentByteCount     *uint64
	keepAliveInterval time.Duration
	listenerChannel   chan *logmessage.Message
	sinkCloseChan     chan Sink
}

func NewWebsocketSink(appId string, givenLogger *gosteno.Logger, ws *websocket.Conn, clientAddress net.Addr, sinkCloseChan chan Sink, keepAliveInterval time.Duration) Sink {
	return &WebsocketSink{
		givenLogger,
		appId,
		ws,
		clientAddress,
		new(uint64),
		new(uint64),
		keepAliveInterval,
		make(chan *logmessage.Message),
		sinkCloseChan,
	}
}

func (sink *WebsocketSink) keepAliveChannel() <-chan bool {
	keepAliveChan := make(chan bool)
	var keepAlive []byte
	go func() {
		for {
			err := websocket.Message.Receive(sink.ws, &keepAlive)
			if err != nil {
				sink.logger.Debugf("Websocket Sink %s: Error receiving keep-alive. Stopping listening. Err: %v", sink.clientAddress, err)
				return
			}
			keepAliveChan <- true
		}
	}()
	return keepAliveChan
}

func (sink *WebsocketSink) Channel() chan *logmessage.Message {
	return sink.listenerChannel
}

func (sink *WebsocketSink) Identifier() string {
	return sink.ws.Request().RemoteAddr
}

func (sink *WebsocketSink) AppId() string {
	return sink.appId
}

func (sink *WebsocketSink) ShouldReceiveErrors() bool {
	return true
}

func (s *WebsocketSink) Logger() *gosteno.Logger {
	return s.logger
}

func (sink *WebsocketSink) Run() {
	sink.logger.Debugf("Websocket Sink %s: Created for appId [%s]", sink.clientAddress, sink.appId)

	keepAliveChan := sink.keepAliveChannel()
	alreadyRequestedClose := false

	buffer := runTruncatingBuffer(sink, 100, sink.Logger())
	for {
		sink.logger.Debugf("Websocket Sink %s: Waiting for activity", sink.clientAddress)
		select {
		case <-keepAliveChan:
			sink.logger.Debugf("Websocket Sink %s: Keep-alive processed", sink.clientAddress)
		case <-time.After(sink.keepAliveInterval):
			sink.logger.Debugf("Websocket Sink %s: No keep keep-alive received. Requesting close.", sink.clientAddress)
			requestClose(sink, sink.sinkCloseChan, &alreadyRequestedClose)
			return
		case message, ok := <-buffer.GetOutputChannel():
			if !ok {
				sink.logger.Debugf("Websocket Sink %s: Closed listener channel detected. Closing websocket", sink.clientAddress)
				sink.ws.Close()
				sink.logger.Debugf("Websocket Sink %s: Websocket successfully closed", sink.clientAddress)
				return
			}
			sink.logger.Debugf("Websocket Sink %s: Got %d bytes. Sending data", sink.clientAddress, message.GetRawMessageLength())
			err := websocket.Message.Send(sink.ws, message.GetRawMessage())
			if err != nil {
				sink.logger.Debugf("Websocket Sink %s: Error when trying to send data to sink %s. Requesting close. Err: %v", sink.clientAddress, err)
				requestClose(sink, sink.sinkCloseChan, &alreadyRequestedClose)
			} else {
				sink.logger.Debugf("Websocket Sink %s: Successfully sent data", sink.clientAddress)
				atomic.AddUint64(sink.sentMessageCount, 1)
				atomic.AddUint64(sink.sentByteCount, uint64(message.GetRawMessageLength()))
			}
		}
	}
}

func (sink *WebsocketSink) Emit() instrumentation.Context {
	return instrumentation.Context{Name: "websocketSink",
		Metrics: []instrumentation.Metric{
			instrumentation.Metric{Name: "sentMessageCount:" + sink.appId, Value: atomic.LoadUint64(sink.sentMessageCount)},
			instrumentation.Metric{Name: "sentByteCount:" + sink.appId, Value: atomic.LoadUint64(sink.sentByteCount)},
		},
	}
}
