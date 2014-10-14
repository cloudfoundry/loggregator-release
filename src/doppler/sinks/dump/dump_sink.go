package dump

import (
	"github.com/cloudfoundry/dropsonde/events"
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
	messageBuffer      []*events.Envelope
	inputChan          chan *events.Envelope
	inactivityDuration time.Duration
	sequence           uint32
	bufferSize         uint32
}

func NewDumpSink(appId string, bufferSize uint32, givenLogger *gosteno.Logger, inactivityDuration time.Duration) *DumpSink {
	dumpSink := &DumpSink{
		appId:              appId,
		logger:             givenLogger,
		messageBuffer:      make([]*events.Envelope, bufferSize),
		inactivityDuration: inactivityDuration,
		bufferSize:         bufferSize,
	}
	return dumpSink
}

func (d *DumpSink) Run(inputChan <-chan *events.Envelope) {
	timer := time.NewTimer(d.inactivityDuration)
	for {
		timer.Reset(d.inactivityDuration)
		select {
		case msg, ok := <-inputChan:
			if !ok {
				return
			}

			_, keepMsg := envelopeTypeWhitelist[msg.GetEventType()]
			if !keepMsg {
				d.logger.Debugf("Dump sink (app id %s): Skipping non-log message (type %s)", d.appId, msg.GetEventType().String())
				continue
			}

			d.addMsg(msg)
		case <-timer.C:
			timer.Stop()
			return
		}
	}
}

func (d *DumpSink) addMsg(msg *events.Envelope) {
	d.Lock()
	defer d.Unlock()
	position := atomic.AddUint32(&d.sequence, uint32(1)) % d.bufferSize
	d.messageBuffer[position] = msg
}

func (d *DumpSink) copyBuffer() (int, []*events.Envelope) {
	d.RLock()
	defer d.RUnlock()
	data := make([]*events.Envelope, d.bufferSize)
	copyCount := copy(data, d.messageBuffer)
	return copyCount, data
}

func (d *DumpSink) Dump() []*events.Envelope {
	sequence := atomic.LoadUint32(&d.sequence)
	out := make([]*events.Envelope, d.bufferSize)
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

func (d *DumpSink) StreamId() string {
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

var envelopeTypeWhitelist = map[events.Envelope_EventType]struct{}{
	events.Envelope_LogMessage: struct{}{},
}
