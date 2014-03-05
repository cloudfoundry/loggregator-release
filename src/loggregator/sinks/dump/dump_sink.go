package dump

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/gunk/timeprovider"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"sync"
	"time"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
)

type DumpSink struct {
	sync.RWMutex
	appId              string
	logger             *gosteno.Logger
	messageBuffer      []*logmessage.Message
	inputChan          chan *logmessage.Message
	inactivityDuration time.Duration
	sequence           uint32
	bufferSize         uint32
	bufferFull		   bool
	tp                 timeprovider.TimeProvider
}

func NewDumpSink(appId string, bufferSize uint32, givenLogger *gosteno.Logger, inactivityDuration time.Duration, tp timeprovider.TimeProvider) *DumpSink {
	dumpSink := &DumpSink{
		appId:              appId,
		logger:             givenLogger,
		messageBuffer:      make([]*logmessage.Message, bufferSize),
		inactivityDuration: inactivityDuration,
		tp:                 tp,
		bufferSize:         bufferSize,
	}
	return dumpSink
}

func (d *DumpSink) Run(inputChan <-chan *logmessage.Message) {
	for {
		countdown := d.tp.NewTickerChannel("", d.inactivityDuration)
		select {
		case msg, ok := <-inputChan:
			if !ok {
				return
			}
			d.addMsg(msg)
		case <-countdown:
			return
		}
	}
}

func (d *DumpSink) addMsg(msg *logmessage.Message) {
	d.Lock()
	defer d.Unlock()

	if d.bufferFull {
		d.messageBuffer = append(d.messageBuffer[1:], msg)
	} else {
		d.messageBuffer = append(d.messageBuffer, msg)
		if uint32(len(d.messageBuffer)) == d.bufferSize {
			d.bufferFull = true
		}
	}
}

func (d *DumpSink) Dump() []*logmessage.Message {
	d.RLock()
	defer d.RUnlock()
	var bufferSize uint32
	if d.bufferFull {
		bufferSize = d.bufferSize
	} else {
		bufferSize = uint32(len(d.messageBuffer))
	}
	result := make([]*logmessage.Message, bufferSize)
	copy(result, d.messageBuffer)
	return result
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
