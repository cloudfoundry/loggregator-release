package dump

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/gunk/timeprovider"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"loggregator/sinks"
	"time"
	"sync/atomic"
	"sync"
)

type DumpSink struct {
	sync.RWMutex
	appId              string
	logger             *gosteno.Logger
	messageBuffer      []*logmessage.Message
	inputChan          chan *logmessage.Message
	timeoutChan        chan sinks.Sink
	inactivityDuration time.Duration
	sequence		   uint32
	bufferSize		   uint32
	tp                 timeprovider.TimeProvider
}

func NewDumpSink(appId string, bufferSize uint32, givenLogger *gosteno.Logger, timeoutChan chan sinks.Sink, inactivityDuration time.Duration, tp timeprovider.TimeProvider) *DumpSink {
	dumpSink := &DumpSink{
		appId:              appId,
		logger:             givenLogger,
		messageBuffer:      make([]*logmessage.Message, bufferSize),
		timeoutChan:        timeoutChan,
		inactivityDuration: inactivityDuration,
		tp:                 tp,
		bufferSize:			bufferSize,
	}
	return dumpSink
}

func (d *DumpSink) Run(inputChan <-chan *logmessage.Message) {
	defer func() {
		d.timeoutChan <- d
	}()
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

func (d *DumpSink) addMsg(msg *logmessage.Message){
	d.Lock()
	defer d.Unlock()
	position := atomic.AddUint32(&d.sequence, uint32(1)) % d.bufferSize
	d.messageBuffer[position] = msg
}

func (d *DumpSink) copyBuffer() (int, []*logmessage.Message) {
	d.RLock()
	defer d.RUnlock()
	data := make([]*logmessage.Message, d.bufferSize)
	copyCount := copy(data, d.messageBuffer)
	return copyCount, data
}

func (d *DumpSink) Dump() []*logmessage.Message {
	sequence := atomic.LoadUint32(&d.sequence)
	out := make([]*logmessage.Message, d.bufferSize)
	_, buffer := d.copyBuffer()

	if d.bufferSize == 1 {
		return buffer
	}

	lapped := d.messageBuffer[0] != nil
	newestPosition := sequence % d.bufferSize
	oldestPosition := (sequence + 1) % d.bufferSize

	if !lapped {
		copyCount:= copy(out, buffer[1:newestPosition+1])
		return out[:copyCount]
	}

	if oldestPosition == 0 {
		copy(out, d.messageBuffer)
	} else {
		copyCount := copy(out, buffer[oldestPosition:])
		copy(out[copyCount:], buffer[:newestPosition+1])
	}
	return out
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
