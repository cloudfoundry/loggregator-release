package websocket

import (
	"doppler/sinks"
	"net"
	"time"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	gorilla "github.com/gorilla/websocket"
)

type remoteMessageWriter interface {
	RemoteAddr() net.Addr
	SetWriteDeadline(time.Time) error
	WriteMessage(messageType int, data []byte) error
}

type WebsocketSink struct {
	logger                 *gosteno.Logger
	streamId               string
	ws                     remoteMessageWriter
	clientAddress          net.Addr
	messageDrainBufferSize uint
	writeTimeout           time.Duration
	dropsondeOrigin        string
}

func NewWebsocketSink(streamId string, givenLogger *gosteno.Logger, ws remoteMessageWriter, messageDrainBufferSize uint, writeTimeout time.Duration, dropsondeOrigin string) *WebsocketSink {
	return &WebsocketSink{
		logger:                 givenLogger,
		streamId:               streamId,
		ws:                     ws,
		clientAddress:          ws.RemoteAddr(),
		messageDrainBufferSize: messageDrainBufferSize,
		writeTimeout:           writeTimeout,
		dropsondeOrigin:        dropsondeOrigin,
	}
}

func (sink *WebsocketSink) Identifier() string {
	return sink.ws.RemoteAddr().String()
}

func (sink *WebsocketSink) StreamId() string {
	return sink.streamId
}

func (sink *WebsocketSink) ShouldReceiveErrors() bool {
	return true
}

func (sink *WebsocketSink) Run(inputChan <-chan *events.Envelope) {
	sink.logger.Debugf("Websocket Sink %s: Running for streamId [%s]", sink.clientAddress, sink.streamId)

	buffer := sinks.RunTruncatingBuffer(inputChan, nil, sink.messageDrainBufferSize, sink.logger, sink.dropsondeOrigin, sink.Identifier(), nil)
	for {
		sink.logger.Debugf("Websocket Sink %s: Waiting for activity", sink.clientAddress)
		messageEnvelope, ok := <-buffer.GetOutputChannel()

		if !ok {
			sink.logger.Debugf("Websocket Sink %s: Closed listener channel detected. Closing websocket", sink.clientAddress)
			return
		}
		messageBytes, err := proto.Marshal(messageEnvelope)

		if err != nil {
			sink.logger.Errorf("Websocket Sink %s: Error marshalling %s envelope from origin %s: %s", sink.clientAddress, messageEnvelope.GetEventType().String(), messageEnvelope.GetOrigin(), err.Error())
			continue
		}

		sink.logger.Debugf("Websocket Sink %s: Received %s message from %s at %d. Sending data.", sink.clientAddress, messageEnvelope.GetEventType().String(), messageEnvelope.GetOrigin(), messageEnvelope.Timestamp)
		if sink.writeTimeout != 0 {
			sink.ws.SetWriteDeadline(time.Now().Add(sink.writeTimeout))
		}
		err = sink.ws.WriteMessage(gorilla.BinaryMessage, messageBytes)
		if err != nil {
			sink.logger.Debugf("Websocket Sink %s: Error when trying to send data to sink %s. Requesting close. Err: %v", sink.clientAddress, err)
			return
		}

		sink.logger.Debugf("Websocket Sink %s: Successfully sent data", sink.clientAddress)
	}
}
