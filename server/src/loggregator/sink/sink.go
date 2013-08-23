package sink

import (
	"code.google.com/p/go.net/websocket"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logtarget"
	"net"
	"sync/atomic"
	"time"
)

type sink struct {
	logger            *gosteno.Logger
	target            *logtarget.LogTarget
	ws                *websocket.Conn
	clientAddress     net.Addr
	sentMessageCount  *uint64
	sentByteCount     *uint64
	keepAliveInterval time.Duration
	listenerChannel   chan []byte
}

func newCfSink(lg *logtarget.LogTarget, givenLogger *gosteno.Logger, ws *websocket.Conn, clientAddress net.Addr, keepAliveInterval time.Duration) *sink {
	return &sink{
		givenLogger,
		lg,
		ws,
		clientAddress,
		new(uint64),
		new(uint64),
		keepAliveInterval,
		make(chan []byte),
	}
}

func (sink *sink) keepAliveChannel() <-chan bool {
	keepAliveChan := make(chan bool)
	var keepAlive []byte
	go func() {
		for {
			err := websocket.Message.Receive(sink.ws, &keepAlive)
			if err != nil {
				sink.logger.Debugf("Sink %s: Error receiving keep-alive. Stopping listening. Err: %v", sink.clientAddress, err)
				return
			}
			keepAliveChan <- true
		}
	}()
	return keepAliveChan
}

func (sink *sink) run(sinkCloseChan chan chan []byte) {
	sink.logger.Debugf("Sink %s: Created for target %s", sink.clientAddress, sink.target)

	keepAliveChan := sink.keepAliveChannel()
	alreadyRequestedClose := false

	for {
		sink.logger.Debugf("Sink %s: Waiting for activity", sink.clientAddress)
		select {
		case <-keepAliveChan:
			sink.logger.Debugf("Sink %s: Keep-alive processed", sink.clientAddress)
		case <-time.After(sink.keepAliveInterval):
			sink.logger.Debugf("Sink %s: No keep keep-alive received. Requesting close.", sink.clientAddress)
			if !alreadyRequestedClose {
				sinkCloseChan <- sink.listenerChannel
				alreadyRequestedClose = true
				sink.logger.Debugf("Sink %s: Successfully requested listener channel close", sink.clientAddress)
			} else {
				sink.logger.Debugf("Sink %s: Previously requested close. Doing nothing", sink.clientAddress)
			}
			return
		case data, ok := <-sink.listenerChannel:
			if !ok {
				sink.logger.Debugf("Sink %s: Closed listener channel detected. Closing websocket", sink.clientAddress)
				sink.ws.Close()
				sink.logger.Debugf("Sink %s: Websocket successfully closed", sink.clientAddress)
				return
			}
			sink.logger.Debugf("Sink %s: Got %d bytes. Sending data", sink.clientAddress, len(data))
			err := websocket.Message.Send(sink.ws, data)
			if err != nil {
				sink.logger.Debugf("Sink %s: Error when trying to send data to sink %s. Requesting close. Err: %v", sink.clientAddress, err)
				if !alreadyRequestedClose {
					sinkCloseChan <- sink.listenerChannel
					alreadyRequestedClose = true
					sink.logger.Debugf("Sink %s: Successfully requested listener channel close", sink.clientAddress)
				} else {
					sink.logger.Debugf("Sink %s: Previously requested close. Doing nothing", sink.clientAddress)
				}
			} else {
				sink.logger.Debugf("Sink %s: Successfully sent data", sink.clientAddress)
			}
			atomic.AddUint64(sink.sentMessageCount, 1)
			atomic.AddUint64(sink.sentByteCount, uint64(len(data)))
		}
	}
}

func (sink *sink) Emit() instrumentation.Context {
	return instrumentation.Context{Name: "cfSink",
		Metrics: []instrumentation.Metric{
			instrumentation.Metric{Name: "sentMessageCount:" + sink.target.Identifier(), Value: atomic.LoadUint64(sink.sentMessageCount)},
			instrumentation.Metric{Name: "sentByteCount:" + sink.target.Identifier(), Value: atomic.LoadUint64(sink.sentByteCount)},
		},
	}
}
