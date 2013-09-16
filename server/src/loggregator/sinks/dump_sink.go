package sinks

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"loggregator/ringbuffer"
	"sync"
)

type DumpSink struct {
	appId      string
	logger     *gosteno.Logger
	inputChan  chan *logmessage.Message
	ringBuffer *ringbuffer.RingBuffer
	bufferSize uint
	*sync.RWMutex
}

func NewDumpSink(appId string, bufferSize uint, givenLogger *gosteno.Logger) *DumpSink {
	dumpSink := &DumpSink{
		appId:      appId,
		logger:     givenLogger,
		inputChan:  make(chan *logmessage.Message),
		bufferSize: bufferSize,
		RWMutex:    &sync.RWMutex{}}
	return dumpSink
}

func (d *DumpSink) Run(sinkCloseChan chan Sink) {
	d.Lock()
	defer d.Unlock()

	d.ringBuffer = runNewRingBuffer(d, d.bufferSize)
}

func (d *DumpSink) Dump() []logmessage.Message {
	d.Lock()
	defer d.Unlock()

	moreMessage := true
	messages := []logmessage.Message{}
	currentMessageChan := d.ringBuffer.GetOutputChannel()
	newMessageChan := make(chan *logmessage.Message, d.bufferSize)
	for moreMessage {
		select {
		case logMessage, ok := <-currentMessageChan:
			newMessageChan <- logMessage
			if ok {
				messages = append(messages, *logMessage)
			}
		default:
			close(currentMessageChan)
			d.ringBuffer.SetOutputChannel(newMessageChan)
			moreMessage = false
		}
	}
	return messages
}

func (d *DumpSink) Channel() chan *logmessage.Message {
	return d.inputChan
}

func (d *DumpSink) AppId() string {
	return d.appId
}

func (d *DumpSink) Logger() *gosteno.Logger {
	return d.logger
}

func (d *DumpSink) Identifier() string {
	return d.appId
}

func (d *DumpSink) Emit() instrumentation.Context {
	return instrumentation.Context{}
}
