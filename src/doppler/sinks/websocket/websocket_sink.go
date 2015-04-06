package websocket

import (
	"doppler/sinks"
	"net"

	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/gosteno"
	"github.com/gogo/protobuf/proto"
	gorilla "github.com/gorilla/websocket"
)

const FIREHOSE_APP_ID = "firehose"

type remoteMessageWriter interface {
	RemoteAddr() net.Addr
	WriteMessage(messageType int, data []byte) error
}

type WebsocketSink struct {
	logger              *gosteno.Logger
	streamId            string
	ws                  remoteMessageWriter
	clientAddress       net.Addr
	wsMessageBufferSize uint
	dropsondeOrigin     string

	sinks.DropCounter
}

func NewWebsocketSink(streamId string, givenLogger *gosteno.Logger, ws remoteMessageWriter, wsMessageBufferSize uint, dropsondeOrigin string, metricUpdateChan chan<- int64) *WebsocketSink {
	return &WebsocketSink{
		logger:              givenLogger,
		streamId:            streamId,
		ws:                  ws,
		clientAddress:       ws.RemoteAddr(),
		wsMessageBufferSize: wsMessageBufferSize,
		dropsondeOrigin:     dropsondeOrigin,
		DropCounter:         sinks.NewDropCounter(streamId, ws.RemoteAddr().String(), metricUpdateChan),
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

	buffer := sinks.RunTruncatingBuffer(inputChan, sink.wsMessageBufferSize, sink.logger, sink.dropsondeOrigin)
	for {
		sink.logger.Debugf("Websocket Sink %s: Waiting for activity", sink.clientAddress)
		messageEnvelope, ok := <-buffer.GetOutputChannel()

		droppedMessages := buffer.GetDroppedMessageCount()
		if droppedMessages != 0 {
			sink.UpdateDroppedMessageCount(droppedMessages)

		}

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
		err = sink.ws.WriteMessage(gorilla.BinaryMessage, messageBytes)
		if err != nil {
			sink.logger.Debugf("Websocket Sink %s: Error when trying to send data to sink %s. Requesting close. Err: %v", sink.clientAddress, err)
			return
		}

		sink.logger.Debugf("Websocket Sink %s: Successfully sent data", sink.clientAddress)
	}
}
