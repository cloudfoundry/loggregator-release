package websocket

import (
	"doppler/sinks"
	"net"
	"sync/atomic"

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
	logger                    *gosteno.Logger
	streamId                  string
	ws                        remoteMessageWriter
	clientAddress             net.Addr
	wsMessageBufferSize       uint
	dropsondeOrigin           string
	droppedMessageCount       uint64
	appDrainMetricsWriterChan chan sinks.DrainMetric
}

func NewWebsocketSink(streamId string, givenLogger *gosteno.Logger, ws remoteMessageWriter, wsMessageBufferSize uint, dropsondeOrigin string, appDrainMetricsWriterChan chan sinks.DrainMetric) *WebsocketSink {
	return &WebsocketSink{
		logger:                    givenLogger,
		streamId:                  streamId,
		ws:                        ws,
		clientAddress:             ws.RemoteAddr(),
		wsMessageBufferSize:       wsMessageBufferSize,
		dropsondeOrigin:           dropsondeOrigin,
		appDrainMetricsWriterChan: appDrainMetricsWriterChan,
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
		sink.UpdateDroppedMessageCount(buffer.GetDroppedMessageCount())
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

func (s *WebsocketSink) UpdateDroppedMessageCount(messageCount uint64) {
	if messageCount == 0 {
		return
	}
	atomic.AddUint64(&s.droppedMessageCount, messageCount)

	metric := sinks.DrainMetric{AppId: s.streamId, DrainURL: s.clientAddress.String(), DroppedMsgCount: atomic.LoadUint64(&s.droppedMessageCount)}

	select {
	case s.appDrainMetricsWriterChan <- metric:
	default:
	}
}
