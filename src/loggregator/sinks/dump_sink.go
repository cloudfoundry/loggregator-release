package sinks

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"loggregator/buffer"
	"sync"
)

type DumpSink struct {
	appId         string
	logger        *gosteno.Logger
	inputChan     chan *logmessage.Message
	messageBuffer buffer.MessageBuffer
	bufferSize    uint
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

func (d *DumpSink) Run() {
	d.Lock()
	defer d.Unlock()

	d.messageBuffer = runNewRingBuffer(d, d.bufferSize, nil)
}

func (d *DumpSink) Dump(outputChan chan *logmessage.Message) {
	d.Lock()
	defer d.Unlock()

	currentMessageChan := d.messageBuffer.GetOutputChannel()
	newMessageChan := make(chan *logmessage.Message, d.bufferSize)

	d.messageBuffer.CloseOutputChannel()
	for message := range currentMessageChan {
		newMessageChan <- message
		select {
		case outputChan <- message:
		default:
			<-outputChan
			outputChan <- message
		}
	}
	close(outputChan)
	d.messageBuffer.SetOutputChannel(newMessageChan)
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
