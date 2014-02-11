package dump

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"loggregator/buffer"
	"time"
	"loggregator/sinks"
)

type DumpSink struct {
	appId              string
	logger             *gosteno.Logger
	messageBuffer      *buffer.DumpableRingBuffer
	inputChan          chan *logmessage.Message
	passThruChan       chan *logmessage.Message
	dumpChan           chan chan []*logmessage.Message
	timeoutChan        chan sinks.Sink
	inactivityDuration time.Duration
}

func NewDumpSink(appId string, bufferSize int, givenLogger *gosteno.Logger, timeoutChan chan sinks.Sink, inactivityDuration time.Duration) *DumpSink {
	inputChan := make(chan *logmessage.Message)
	passThruChan := make(chan *logmessage.Message)
	dumpChan := make(chan chan []*logmessage.Message)

	dumpSink := &DumpSink{
		appId:              appId,
		logger:             givenLogger,
		inputChan:          inputChan,
		passThruChan:       passThruChan,
		messageBuffer:      buffer.NewDumpableRingBuffer(passThruChan, bufferSize),
		dumpChan:           dumpChan,
		timeoutChan:        timeoutChan,
		inactivityDuration: inactivityDuration,
	}
	return dumpSink
}

func (d *DumpSink) Run() {
	defer func() {
		d.timeoutChan <- d
	}()
	for {
		countdown := time.After(d.inactivityDuration)
		select {
		case msg, ok := <-d.inputChan:
			if !ok {
				close(d.passThruChan)
				return
			}
			d.passThruChan <- msg
		case dump, ok := <-d.dumpChan:
			if !ok {
				return
			}
			dump <- d.messageBuffer.Dump()
		case <-countdown:
			return
		}
	}
}

func (d *DumpSink) Dump() []*logmessage.Message {
	dump := make(chan []*logmessage.Message)
	d.dumpChan <- dump
	return <-dump
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
