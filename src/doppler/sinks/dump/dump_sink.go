package dump

import (
	"doppler/envelopewrapper"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"sync"
	"sync/atomic"
	"time"
)

type DumpSink struct {
	sync.RWMutex
	appId              string
	logger             *gosteno.Logger
	messageBuffer      []*envelopewrapper.WrappedEnvelope
	inputChan          chan *envelopewrapper.WrappedEnvelope
	inactivityDuration time.Duration
	sequence           uint32
	bufferSize         uint32
}

func NewDumpSink(appId string, bufferSize uint32, givenLogger *gosteno.Logger, inactivityDuration time.Duration) *DumpSink {
	dumpSink := &DumpSink{
		appId:              appId,
		logger:             givenLogger,
		messageBuffer:      make([]*envelopewrapper.WrappedEnvelope, bufferSize),
		inactivityDuration: inactivityDuration,
		bufferSize:         bufferSize,
	}
	return dumpSink
}

func (d *DumpSink) Run(inputChan <-chan *envelopewrapper.WrappedEnvelope) {
	timer := time.NewTimer(d.inactivityDuration)
	for {
		timer.Reset(d.inactivityDuration)
		select {
		case msg, ok := <-inputChan:
			if !ok {
				return
			}
			d.addMsg(msg)
		case <-timer.C:
			timer.Stop()
			return
		}
	}
}

func (d *DumpSink) addMsg(msg *envelopewrapper.WrappedEnvelope) {
	d.Lock()
	defer d.Unlock()
	position := atomic.AddUint32(&d.sequence, uint32(1)) % d.bufferSize
	d.messageBuffer[position] = msg
}

func (d *DumpSink) copyBuffer() (int, []*envelopewrapper.WrappedEnvelope) {
	d.RLock()
	defer d.RUnlock()
	data := make([]*envelopewrapper.WrappedEnvelope, d.bufferSize)
	copyCount := copy(data, d.messageBuffer)
	return copyCount, data
}

func (d *DumpSink) Dump() []*envelopewrapper.WrappedEnvelope {
	sequence := atomic.LoadUint32(&d.sequence)
	out := make([]*envelopewrapper.WrappedEnvelope, d.bufferSize)
	_, buffer := d.copyBuffer()

	if d.bufferSize == 1 {
		return buffer
	}

	lapped := d.messageBuffer[0] != nil
	newestPosition := sequence % d.bufferSize
	oldestPosition := (sequence + 1) % d.bufferSize

	if !lapped {
		copyCount := copy(out, buffer[1:newestPosition+1])
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
