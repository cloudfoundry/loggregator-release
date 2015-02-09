package truncatingbuffer

import (
	"doppler/buffer"
	"fmt"
	"github.com/cloudfoundry/dropsonde/envelope_extensions"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/gosteno"
	"github.com/gogo/protobuf/proto"
	"sync"
	"time"
)

type truncatingBuffer struct {
	inputChannel    <-chan *events.Envelope
	outputChannel   chan *events.Envelope
	logger          *gosteno.Logger
	lock            *sync.RWMutex
	dropsondeOrigin string
}

func NewTruncatingBuffer(inputChannel <-chan *events.Envelope, bufferSize uint, logger *gosteno.Logger, dropsondeOrigin string) buffer.MessageBuffer {
	outputChannel := make(chan *events.Envelope, bufferSize)
	return &truncatingBuffer{
		inputChannel:    inputChannel,
		outputChannel:   outputChannel,
		logger:          logger,
		lock:            &sync.RWMutex{},
		dropsondeOrigin: dropsondeOrigin,
	}
}

func (r *truncatingBuffer) GetOutputChannel() <-chan *events.Envelope {
	r.lock.Lock()
	defer r.lock.Unlock()

	return r.outputChannel
}

func (r *truncatingBuffer) CloseOutputChannel() {
	close(r.outputChannel)
}

func (r *truncatingBuffer) Run() {
	for msg := range r.inputChannel {
		r.lock.Lock()
		select {
		case r.outputChannel <- msg:
		default:
			messageCount := len(r.outputChannel)
			r.outputChannel = make(chan *events.Envelope, cap(r.outputChannel))
			appId := envelope_extensions.GetAppId(msg)
			lm := generateLogMessage(fmt.Sprintf("Log message output too high. We've dropped %d messages", messageCount), appId)

			env := &events.Envelope{
				EventType:  events.Envelope_LogMessage.Enum(),
				LogMessage: lm,
				Origin:     proto.String(r.dropsondeOrigin),
			}

			r.outputChannel <- env
			r.outputChannel <- msg

			if r.logger != nil {
				r.logger.Warn(fmt.Sprintf("TB: Output channel too full. Dropped %d messages for app %s.", messageCount, appId))
			}
		}
		r.lock.Unlock()
	}
	close(r.outputChannel)
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
