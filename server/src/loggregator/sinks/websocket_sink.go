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

type websocketSink struct {
	logger            *gosteno.Logger
	appId             string
	ws                *websocket.Conn
	clientAddress     net.Addr
	sentMessageCount  *uint64
	sentByteCount     *uint64
	keepAliveInterval time.Duration
	listenerChannel   chan *logmessage.Message
}

func NewWebsocketSink(appId string, givenLogger *gosteno.Logger, ws *websocket.Conn, clientAddress net.Addr, keepAliveInterval time.Duration) Sink {
	return &websocketSink{
		givenLogger,
		appId,
		ws,
		clientAddress,
		new(uint64),
		new(uint64),
		keepAliveInterval,
		make(chan *logmessage.Message),
	}
}

func (sink *websocketSink) keepAliveChannel() <-chan bool {
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

func (sink *websocketSink) ListenerChannel() chan *logmessage.Message {
	return sink.listenerChannel
}

func (sink *websocketSink) Identifier() string {
	return sink.clientAddress.String()
}

func (sink *websocketSink) Run(sinkCloseChan chan chan *logmessage.Message) {
	sink.logger.Debugf("Websocket Sink %s: Created for appId [%s]", sink.clientAddress, sink.appId)

	keepAliveChan := sink.keepAliveChannel()
	alreadyRequestedClose := false

	outMessageChan := make(chan *logmessage.Message, 10)
	go OverwritingMessageChannel(sink.listenerChannel, outMessageChan, sink.logger)

	for {
		sink.logger.Debugf("Websocket Sink %s: Waiting for activity", sink.clientAddress)
		select {
		case <-keepAliveChan:
			sink.logger.Debugf("Websocket Sink %s: Keep-alive processed", sink.clientAddress)
		case <-time.After(sink.keepAliveInterval):
			sink.logger.Debugf("Websocket Sink %s: No keep keep-alive received. Requesting close.", sink.clientAddress)
			if !alreadyRequestedClose {
				sinkCloseChan <- sink.listenerChannel
				alreadyRequestedClose = true
				sink.logger.Debugf("Websocket Sink %s: Successfully requested listener channel close", sink.clientAddress)
			} else {
				sink.logger.Debugf("Websocket Sink %s: Previously requested close. Doing nothing", sink.clientAddress)
			}
			return
		case message, ok := <-outMessageChan:
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
				if !alreadyRequestedClose {
					sinkCloseChan <- sink.listenerChannel
					alreadyRequestedClose = true
					sink.logger.Debugf("Websocket Sink %s: Successfully requested listener channel close", sink.clientAddress)
				} else {
					sink.logger.Debugf("Websocket Sink %s: Previously requested close. Doing nothing", sink.clientAddress)
				}
			} else {
				sink.logger.Debugf("Websocket Sink %s: Successfully sent data", sink.clientAddress)
				atomic.AddUint64(sink.sentMessageCount, 1)
				atomic.AddUint64(sink.sentByteCount, uint64(message.GetRawMessageLength()))
			}
		}
	}
}

func (sink *websocketSink) Emit() instrumentation.Context {
	return instrumentation.Context{Name: "websocketSink",
		Metrics: []instrumentation.Metric{
			instrumentation.Metric{Name: "sentMessageCount:" + sink.appId, Value: atomic.LoadUint64(sink.sentMessageCount)},
			instrumentation.Metric{Name: "sentByteCount:" + sink.appId, Value: atomic.LoadUint64(sink.sentByteCount)},
		},
	}
}
