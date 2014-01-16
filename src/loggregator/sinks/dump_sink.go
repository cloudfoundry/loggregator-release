package sinks

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"loggregator/buffer"
)

type DumpSink struct {
	appId         string
	logger        *gosteno.Logger
	messageBuffer *buffer.DumpableRingBuffer
	inputChan     chan *logmessage.Message
}

func NewDumpSink(appId string, bufferSize int, givenLogger *gosteno.Logger) *DumpSink {
	inputChan := make(chan *logmessage.Message)
	dumpSink := &DumpSink{
		appId:         appId,
		logger:        givenLogger,
		inputChan:     inputChan,
		messageBuffer: buffer.NewDumpableRingBuffer(inputChan, bufferSize),
	}
	return dumpSink
}

func (d *DumpSink) Run() {
	// no-op to conform to Sink interface
}

func (d *DumpSink) Dump(in chan<- *logmessage.Message) {
	d.messageBuffer.Dump(in)
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

func (d *DumpSink) ShouldReceiveErrors() bool {
	return true
}
