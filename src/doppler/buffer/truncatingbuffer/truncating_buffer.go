package truncatingbuffer

import (
	"code.google.com/p/gogoprotobuf/proto"
	"doppler/buffer"
	"doppler/envelopewrapper"
	"fmt"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/gosteno"
	"sync"
	"time"
)

type truncatingBuffer struct {
	inputChannel  <-chan *envelopewrapper.WrappedEnvelope
	outputChannel chan *envelopewrapper.WrappedEnvelope
	logger        *gosteno.Logger
	lock          *sync.RWMutex
}

func NewTruncatingBuffer(inputChannel <-chan *envelopewrapper.WrappedEnvelope, bufferSize uint, logger *gosteno.Logger) buffer.MessageBuffer {
	outputChannel := make(chan *envelopewrapper.WrappedEnvelope, bufferSize)
	return &truncatingBuffer{inputChannel, outputChannel, logger, &sync.RWMutex{}}
}

func (r *truncatingBuffer) GetOutputChannel() <-chan *envelopewrapper.WrappedEnvelope {
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
			r.outputChannel = make(chan *envelopewrapper.WrappedEnvelope, cap(r.outputChannel))
			lm := generateLogMessage(fmt.Sprintf("Log message output too high. We've dropped %d messages", messageCount), msg.Envelope.GetAppId())

			env := &events.Envelope{
				EventType:  events.Envelope_LogMessage.Enum(),
				LogMessage: lm,
				Origin:     proto.String("FIXME"),
			}
			envBytes, err := proto.Marshal(env)

			if err != nil {
				r.logger.Error("TB: Output channel too full. And we failed to notify them. Dropping Buffer.")
				continue
			}

			r.outputChannel <- &envelopewrapper.WrappedEnvelope{Envelope: env, EnvelopeBytes: envBytes}
			r.outputChannel <- msg

			if r.logger != nil {
				r.logger.Warn("TB: Output channel too full. Dropped Buffer.")
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
