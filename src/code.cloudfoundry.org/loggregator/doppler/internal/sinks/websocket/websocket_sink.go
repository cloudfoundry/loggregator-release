package websocket

import (
	"log"
	"net"
	"time"

	"code.cloudfoundry.org/loggregator/doppler/internal/sinks"

	"code.cloudfoundry.org/loggregator/doppler/internal/truncatingbuffer"

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
	appID                  string
	ws                     remoteMessageWriter
	clientAddress          net.Addr
	messageDrainBufferSize uint
	writeTimeout           time.Duration
	dropsondeOrigin        string
	counter                Counter
}

func NewWebsocketSink(appID string, ws remoteMessageWriter, messageDrainBufferSize uint, writeTimeout time.Duration, dropsondeOrigin string) *WebsocketSink {
	return &WebsocketSink{
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
	log.Printf("Websocket Sink %s: Running for streamId [%s]", sink.clientAddress, sink.appID)
	context := truncatingbuffer.NewDefaultContext(sink.dropsondeOrigin, sink.Identifier())
	buffer := sinks.RunTruncatingBuffer(inputChan, sink.messageDrainBufferSize, context, stopChan)
	for {
		messageEnvelope, ok := <-buffer.GetOutputChannel()

		if !ok {
			log.Printf("Websocket Sink %s: Closed listener channel detected. Closing websocket", sink.clientAddress)
			close(stopChan)
			return
		}

		messageBytes, err := proto.Marshal(messageEnvelope)
		if err != nil {
			log.Printf("Websocket Sink %s: Error marshalling %s envelope from origin %s: %s", sink.clientAddress, messageEnvelope.GetEventType(), messageEnvelope.GetOrigin(), err.Error())
			continue
		}

		if sink.writeTimeout != 0 {
			sink.ws.SetWriteDeadline(time.Now().Add(sink.writeTimeout))
		}
		err = sink.ws.WriteMessage(gorilla.BinaryMessage, messageBytes)
		if err != nil {
			log.Printf("Websocket Sink %s: Error when trying to send data to sink. Requesting close. Err: %v", sink.clientAddress, err)
			close(stopChan)
			return
		}

		sink.counter.Increment(messageEnvelope.GetEventType())
	}
}
