package truncatingbuffer

import (
	"fmt"
	"sync"
	"time"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/envelope_extensions"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

type TruncatingBuffer struct {
	inputChannel        <-chan *events.Envelope
	outputChannel       chan *events.Envelope
	logger              *gosteno.Logger
	lock                *sync.RWMutex
	dropsondeOrigin     string
	droppedMessageCount int64
}

func NewTruncatingBuffer(inputChannel <-chan *events.Envelope, bufferSize uint, logger *gosteno.Logger, dropsondeOrigin string) *TruncatingBuffer {
	outputChannel := make(chan *events.Envelope, bufferSize)
	return &TruncatingBuffer{
		inputChannel:        inputChannel,
		outputChannel:       outputChannel,
		logger:              logger,
		lock:                &sync.RWMutex{},
		dropsondeOrigin:     dropsondeOrigin,
		droppedMessageCount: 0,
	}
}

func (r *TruncatingBuffer) GetOutputChannel() <-chan *events.Envelope {
	r.lock.Lock()
	defer r.lock.Unlock()

	return r.outputChannel
}

func (r *TruncatingBuffer) CloseOutputChannel() {
	close(r.outputChannel)
}

func (r *TruncatingBuffer) Run() {
	for msg := range r.inputChannel {
		r.lock.Lock()
		select {
		case r.outputChannel <- msg:
		default:
			messageCount := len(r.outputChannel)
			r.droppedMessageCount += int64(messageCount)
			r.outputChannel = make(chan *events.Envelope, cap(r.outputChannel))
			appId := envelope_extensions.GetAppId(msg)
			lm := generateLogMessage(fmt.Sprintf("Log message output too high. We've dropped %d messages", messageCount), appId)

			env, err := emitter.Wrap(lm, r.dropsondeOrigin)
			if err == nil {
				r.outputChannel <- env
			} else {
				r.logger.Warnf("Error marshalling message: %v", err)
			}

			r.outputChannel <- msg

			if r.logger != nil {
				r.logger.Warn(fmt.Sprintf("TB: Output channel too full. Dropped %d messages for app %s.", messageCount, appId))
			}
		}
		r.lock.Unlock()
	}
	close(r.outputChannel)
}

func (r *TruncatingBuffer) GetDroppedMessageCount() int64 {
	r.lock.RLock()
	defer r.lock.RUnlock()
	messages := r.droppedMessageCount
	r.droppedMessageCount = 0
	return messages
}

func generateLogMessage(messageString string, appId string) *events.LogMessage {
	messageType := events.LogMessage_ERR
	currentTime := time.Now()
	logMessage := &events.LogMessage{
		Message:     []byte(messageString),
		AppId:       &appId,
		MessageType: &messageType,
		SourceType:  proto.String("LGR"),
		Timestamp:   proto.Int64(currentTime.UnixNano()),
	}

	return logMessage
}
