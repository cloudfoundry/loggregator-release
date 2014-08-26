package websocket

import (
	"doppler/envelopewrapper"
	"doppler/sinks"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	gorilla "github.com/gorilla/websocket"
	"net"
	"sync/atomic"
)

type remoteMessageWriter interface {
	RemoteAddr() net.Addr
	WriteMessage(messageType int, data []byte) error
}

type WebsocketSink struct {
	logger              *gosteno.Logger
	appId               string
	ws                  remoteMessageWriter
	clientAddress       net.Addr
	sentMessageCount    uint64
	sentByteCount       uint64
	wsMessageBufferSize uint
	dropsondeOrigin     string
}

func NewWebsocketSink(appId string, givenLogger *gosteno.Logger, ws remoteMessageWriter, wsMessageBufferSize uint, dropsondeOrigin string) *WebsocketSink {
	return &WebsocketSink{
		logger:              givenLogger,
		appId:               appId,
		ws:                  ws,
		clientAddress:       ws.RemoteAddr(),
		wsMessageBufferSize: wsMessageBufferSize,
		dropsondeOrigin:     dropsondeOrigin,
	}
}

func (sink *WebsocketSink) Identifier() string {
	return sink.ws.RemoteAddr().String()
}

func (sink *WebsocketSink) AppId() string {
	return sink.appId
}

func (sink *WebsocketSink) ShouldReceiveErrors() bool {
	return true
}

func (sink *WebsocketSink) Run(inputChan <-chan *envelopewrapper.WrappedEnvelope) {
	sink.logger.Debugf("Websocket Sink %s: Running for appId [%s]", sink.clientAddress, sink.appId)

	buffer := sinks.RunTruncatingBuffer(inputChan, sink.wsMessageBufferSize, sink.logger, sink.dropsondeOrigin)
	for {
		sink.logger.Debugf("Websocket Sink %s: Waiting for activity", sink.clientAddress)
		message, ok := <-buffer.GetOutputChannel()
		if !ok {
			sink.logger.Debugf("Websocket Sink %s: Closed listener channel detected. Closing websocket", sink.clientAddress)
			return
		}
		sink.logger.Debugf("Websocket Sink %s: Got %d bytes. Sending data", sink.clientAddress, message.EnvelopeLength())
		err := sink.ws.WriteMessage(gorilla.BinaryMessage, message.EnvelopeBytes)
		if err != nil {
			sink.logger.Debugf("Websocket Sink %s: Error when trying to send data to sink %s. Requesting close. Err: %v", sink.clientAddress, err)
			return
		}

		sink.logger.Debugf("Websocket Sink %s: Successfully sent data", sink.clientAddress)
		atomic.AddUint64(&sink.sentMessageCount, 1)
		atomic.AddUint64(&sink.sentByteCount, uint64(message.EnvelopeLength()))
	}
}

func (sink *WebsocketSink) Emit() instrumentation.Context {
	return instrumentation.Context{Name: "websocketSink",
		Metrics: []instrumentation.Metric{
			instrumentation.Metric{Name: "sentMessageCount:" + sink.appId, Value: atomic.LoadUint64(&sink.sentMessageCount)},
			instrumentation.Metric{Name: "sentByteCount:" + sink.appId, Value: atomic.LoadUint64(&sink.sentByteCount)},
		},
	}
}
