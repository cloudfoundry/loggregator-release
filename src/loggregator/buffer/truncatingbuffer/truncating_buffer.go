package truncatingbuffer

import (
	"code.google.com/p/gogoprotobuf/proto"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"loggregator/buffer"
	"sync"
	"time"
)

type truncatingBuffer struct {
	inputChannel  <-chan *logmessage.Message
	outputChannel chan *logmessage.Message
	logger        *gosteno.Logger
	lock          *sync.RWMutex
}

func NewTruncatingBuffer(inputChannel <-chan *logmessage.Message, bufferSize uint, logger *gosteno.Logger) buffer.MessageBuffer {
	outputChannel := make(chan *logmessage.Message, bufferSize)
	return &truncatingBuffer{inputChannel, outputChannel, logger, &sync.RWMutex{}}
}

func (r *truncatingBuffer) SetOutputChannel(out chan *logmessage.Message) {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.outputChannel = out
}

func (r *truncatingBuffer) GetOutputChannel() <-chan *logmessage.Message {
	r.lock.Lock()
	defer r.lock.Unlock()

	return r.outputChannel
}

func (r *truncatingBuffer) CloseOutputChannel() {
	close(r.outputChannel)
}

func (r *truncatingBuffer) Run() {
	for v := range r.inputChannel {
		r.lock.Lock()
		select {
		case r.outputChannel <- v:
		default:
			messageCount := len(r.outputChannel)
			r.outputChannel = make(chan *logmessage.Message, cap(r.outputChannel))
			lm := generateLogMessage(fmt.Sprintf("We've truncated %d messages", messageCount), v.GetLogMessage().AppId)
			lmBytes, err := proto.Marshal(lm)
			if err != nil {
				r.logger.Info("RBC: Reader was too slow. And we failed to notify them. Dropping Buffer.")
				continue
			}
			r.outputChannel <- logmessage.NewMessage(lm, lmBytes)
			r.outputChannel <- v
			if r.logger != nil {
				r.logger.Warnf("RBC: Reader was too slow. Dropped Buffer.")
			}
		}
		r.lock.Unlock()
	}
	close(r.outputChannel)
}

func generateLogMessage(messageString string, appId *string) *logmessage.LogMessage {
	messageType := logmessage.LogMessage_ERR
	currentTime := time.Now()
	logMessage := &logmessage.LogMessage{
		Message:     []byte(messageString),
		AppId:       appId,
		MessageType: &messageType,
		SourceName:  proto.String("LGR"),
		Timestamp:   proto.Int64(currentTime.UnixNano()),
	}

	return logMessage
}
