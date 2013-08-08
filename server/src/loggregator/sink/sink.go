package sink

import (
	"cfcomponent/instrumentation"
	"code.google.com/p/go.net/websocket"
	"github.com/cloudfoundry/gosteno"
	"net"
	"strings"
	"sync/atomic"
	"time"
)

type sink struct {
	logger            *gosteno.Logger
	spaceId           string
	appId             string
	ws                *websocket.Conn
	clientAddress     net.Addr
	sentMessageCount  *uint64
	sentByteCount     *uint64
	keepAliveInterval time.Duration
	listenerChannel   chan []byte
}

func newCfSink(spaceId string, appId string, givenLogger *gosteno.Logger, ws *websocket.Conn, clientAddress net.Addr, keepAliveInterval time.Duration) *sink {
	return &sink{
		givenLogger,
		spaceId,
		appId,
		ws,
		clientAddress,
		new(uint64),
		new(uint64),
		keepAliveInterval,
		make(chan []byte),
	}
}

func (sink *sink) clientIdentifier() string {
	return strings.Join([]string{sink.spaceId, sink.appId}, ":")
}

func (sink *sink) Run(sinkCloseChan chan chan []byte) {
	if sink.appId != "" {
		sink.logger.Debugf("Adding Tail client %s for space [%s] and app [%s].", sink.clientAddress, sink.spaceId, sink.appId)
	} else {
		sink.logger.Debugf("Adding Tail client %s for space [%s].", sink.clientAddress, sink.spaceId)
	}

	alreadyAskedForClose := false

	keepAliveChan := make(chan []byte)
	go func() {
		for {
			var keepAlive []byte
			err := websocket.Message.Receive(sink.ws, &keepAlive)
			if err != nil {
				sink.logger.Debugf("Error receiving keep-alive. %v for %v", err, sink.clientAddress)
				break
			}
			sink.logger.Debugf("Received a keep-alive for %v", sink.clientAddress)
			keepAliveChan <- keepAlive
		}
	}()

	go func() {
		for {
			sink.logger.Debugf("Waiting for keep-alive for %v", sink.clientAddress)
			select {
			case <-keepAliveChan:
				sink.logger.Debugf("Keep-alive processed for %v", sink.clientAddress)
			case <-time.After(sink.keepAliveInterval):
				sinkCloseChan <- sink.listenerChannel
				alreadyAskedForClose = true
				return
			}
		}
	}()

	for {
		sink.logger.Debugf("Tail client %s is waiting for data", sink.clientAddress)
		data, ok := <-sink.listenerChannel
		if !ok {
			sink.ws.Close()
			sink.logger.Debug("Sink client channel closed.")
			return
		}
		sink.logger.Debugf("Tail client %s got %d bytes", sink.clientAddress, len(data))
		err := websocket.Message.Send(sink.ws, data)
		if err != nil {
			sink.logger.Debugf("Error when sending data to sink %s. Err: %v", sink.clientAddress, err)
			if !alreadyAskedForClose {
				sinkCloseChan <- sink.listenerChannel
				alreadyAskedForClose = true
			}
		}
		atomic.AddUint64(sink.sentMessageCount, 1)
		atomic.AddUint64(sink.sentByteCount, uint64(len(data)))
	}
}

func (sink *sink) Emit() instrumentation.Context {
	return instrumentation.Context{"cfSink",
		[]instrumentation.Metric{
			instrumentation.Metric{"sentMessageCount:" + sink.clientIdentifier(), atomic.LoadUint64(sink.sentMessageCount)},
			instrumentation.Metric{"sentByteCount:" + sink.clientIdentifier(), atomic.LoadUint64(sink.sentByteCount)},
		},
	}
}
