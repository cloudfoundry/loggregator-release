package websocket

import (
	"doppler/sinks"
	"net"
	"time"

	"truncatingbuffer"

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

type Counter interface {
	Increment(events.Envelope_EventType)
}

type noopCounter struct{}

func (noopCounter) Increment(events.Envelope_EventType) {}

type WebsocketSink struct {
	logger                 *gosteno.Logger
	appID                  string
	ws                     remoteMessageWriter
	clientAddress          net.Addr
	messageDrainBufferSize uint
	writeTimeout           time.Duration
	dropsondeOrigin        string
	counter                Counter
}

func NewWebsocketSink(appID string, givenLogger *gosteno.Logger, ws remoteMessageWriter, messageDrainBufferSize uint, writeTimeout time.Duration, dropsondeOrigin string) *WebsocketSink {
	return &WebsocketSink{
		logger:                 givenLogger,
		appID:                  appID,
		ws:                     ws,
		clientAddress:          ws.RemoteAddr(),
		messageDrainBufferSize: messageDrainBufferSize,
		writeTimeout:           writeTimeout,
		dropsondeOrigin:        dropsondeOrigin,
		counter:                noopCounter{},
	}
}

func (sink *WebsocketSink) SetCounter(counter Counter) {
	sink.counter = counter
}

func (sink *WebsocketSink) Identifier() string {
	return sink.ws.RemoteAddr().String()
}

func (sink *WebsocketSink) AppID() string {
	return sink.appID
}

func (sink *WebsocketSink) ShouldReceiveErrors() bool {
	return true
}

func (sink *WebsocketSink) Run(inputChan <-chan *events.Envelope) {
	stopChan := make(chan struct{})
	sink.logger.Debugf("Websocket Sink %s: Running for streamId [%s]", sink.clientAddress, sink.appID)
	context := truncatingbuffer.NewDefaultContext(sink.dropsondeOrigin, sink.Identifier())
	buffer := sinks.RunTruncatingBuffer(inputChan, sink.messageDrainBufferSize, context, sink.logger, stopChan)
	for {
		sink.logger.Debugf("Websocket Sink %s: Waiting for activity", sink.clientAddress)
		messageEnvelope, ok := <-buffer.GetOutputChannel()

		if !ok {
			sink.logger.Debugf("Websocket Sink %s: Closed listener channel detected. Closing websocket", sink.clientAddress)
			close(stopChan)
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
			sink.logger.Debugf("Websocket Sink %s: Error when trying to send data to sink. Requesting close. Err: %v", sink.clientAddress, err)
			close(stopChan)
			return
		}

		sink.logger.Debugf("Websocket Sink %s: Successfully sent data", sink.clientAddress)
		sink.counter.Increment(messageEnvelope.GetEventType())
	}
}
