package truncatingbuffer

import (
	"fmt"
	"sync"
	"time"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

var lgrSource = proto.String("LGR")

type TruncatingBuffer struct {
	inputChannel               <-chan *events.Envelope
	context                    BufferContext
	outputChannel              chan *events.Envelope
	logger                     *gosteno.Logger
	lock                       *sync.RWMutex
	bufferSize                 uint64
	sentMessageCount           uint64
	queuedInternalMessageCount uint64
	droppedMessageCount        uint64
	stopChannel                chan struct{}
}

func NewTruncatingBuffer(inputChannel <-chan *events.Envelope, bufferSize uint, context BufferContext, logger *gosteno.Logger, stopChannel chan struct{}) *TruncatingBuffer {
	if bufferSize < 3 {
		panic("bufferSize must be larger than 3 for overflow")
	}
	if context == nil {
		panic("context should not be nil")
	}
	return &TruncatingBuffer{
		inputChannel:               inputChannel,
		outputChannel:              make(chan *events.Envelope, bufferSize),
		logger:                     logger,
		lock:                       &sync.RWMutex{},
		bufferSize:                 uint64(bufferSize),
		sentMessageCount:           0,
		queuedInternalMessageCount: 0,
		droppedMessageCount:        0,
		stopChannel:                stopChannel,
		context:                    context,
	}
}

func (r *TruncatingBuffer) GetOutputChannel() <-chan *events.Envelope {
	return r.outputChannel
}

func (r *TruncatingBuffer) closeOutputChannel() {
	close(r.outputChannel)
}

func (r *TruncatingBuffer) eventAllowed(eventType events.Envelope_EventType) bool {
	return r.context.EventAllowed(eventType)
}

func (r *TruncatingBuffer) Run() {
	defer r.closeOutputChannel()
	for {
		select {
		case <-r.stopChannel:
			return
		case msg, ok := <-r.inputChannel:
			if !ok {
				return
			}
			if r.eventAllowed(msg.GetEventType()) {
				r.forwardMessage(msg)
			}
		}
	}
}

func (r *TruncatingBuffer) forwardMessage(msg *events.Envelope) {
	select {
	case r.outputChannel <- msg:
		r.sentMessageCount++
		queuedInternalMessageWasSent := (r.sentMessageCount+r.queuedInternalMessageCount > r.bufferSize)
		if r.queuedInternalMessageCount > 0 && queuedInternalMessageWasSent {
			r.queuedInternalMessageCount--
		}

	default:
		deltaDropped := (r.dropMessages() - r.queuedInternalMessageCount)
		r.queuedInternalMessageCount = 0

		r.lock.Lock()
		r.droppedMessageCount += deltaDropped
		appId := r.context.AppID(msg)
		r.notifyMessagesDropped(r.outputChannel, deltaDropped, r.droppedMessageCount, appId)
		totalDropped := r.droppedMessageCount
		r.lock.Unlock()

		r.outputChannel <- msg
		r.sentMessageCount = 1

		if r.logger != nil {
			r.logger.Warnd(map[string]interface{}{
				"dropped":       deltaDropped,
				"total_dropped": totalDropped,
				"appId":         appId,
				"destination":   r.context.Destination(),
			}, "TB: Output channel too full")
		}
	}
}

func (r *TruncatingBuffer) dropMessages() uint64 {
	dropped := uint64(0)
	for {
		select {
		case _, ok := <-r.outputChannel:
			if !ok {
				return dropped
			}
			dropped++
		default:
			return dropped
		}
	}
}

func (r *TruncatingBuffer) GetDroppedMessageCount() uint64 {
	r.lock.RLock()
	defer r.lock.RUnlock()
	messages := r.droppedMessageCount
	r.droppedMessageCount = 0
	return messages
}

func (r *TruncatingBuffer) PeekDroppedMessageCount() uint64 {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return r.droppedMessageCount
}

func (r *TruncatingBuffer) notifyMessagesDropped(outputChannel chan *events.Envelope, deltaDropped, totalDropped uint64, appId string) {
	metrics.BatchAddCounter("TruncatingBuffer.totalDroppedMessages", deltaDropped)
	if r.eventAllowed(events.Envelope_LogMessage) {
		r.emitMessage(outputChannel, generateLogMessage(deltaDropped, totalDropped, appId, r.context.Origin(), r.context.Destination()))
	}
	if r.eventAllowed(events.Envelope_CounterEvent) {
		r.emitMessage(outputChannel, generateCounterEvent(deltaDropped, totalDropped))
	}
}

func (r *TruncatingBuffer) emitMessage(outputChannel chan *events.Envelope, event events.Event) {
	env, err := emitter.Wrap(event, r.context.Origin())
	if err == nil {
		outputChannel <- env
		r.queuedInternalMessageCount++
	} else {
		r.logger.Warnf("Error marshalling message: %v", err)
	}
}

func generateLogMessage(deltaDropped, totalDropped uint64, appId, source, destination string) *events.LogMessage {
	messageString := fmt.Sprintf("Log message output is too high. %d messages dropped (Total %d messages dropped) from %s to %s.", deltaDropped, totalDropped, source, destination)

	messageType := events.LogMessage_ERR
	currentTime := time.Now()
	logMessage := &events.LogMessage{
		Message:     []byte(messageString),
		AppId:       &appId,
		MessageType: &messageType,
		SourceType:  lgrSource,
		Timestamp:   proto.Int64(currentTime.UnixNano()),
	}

	return logMessage
}

func generateCounterEvent(delta, total uint64) *events.CounterEvent {
	return &events.CounterEvent{
		Name:  proto.String("TruncatingBuffer.DroppedMessages"),
		Delta: proto.Uint64(delta),
		Total: proto.Uint64(total),
	}
}
