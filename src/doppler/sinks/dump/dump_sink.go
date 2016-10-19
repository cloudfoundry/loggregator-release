package dump

import (
	"container/ring"
	"sync"
	"time"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
)

type DumpSink struct {
	appId              string
	logger             *gosteno.Logger
	messageRing        *ring.Ring
	inputChan          chan *events.Envelope
	inactivityDuration time.Duration
	lock               sync.RWMutex
}

func NewDumpSink(appId string, bufferSize uint32, givenLogger *gosteno.Logger, inactivityDuration time.Duration) *DumpSink {
	dumpSink := &DumpSink{
		appId:              appId,
		logger:             givenLogger,
		messageRing:        ring.New(int(bufferSize)),
		inactivityDuration: inactivityDuration,
	}
	return dumpSink
}

func (d *DumpSink) Run(inputChan <-chan *events.Envelope) {
	timer := time.NewTimer(d.inactivityDuration)
	defer timer.Stop()
	for {
		select {
		case msg, ok := <-inputChan:
			if !ok {
				return
			}

			if msg.GetEventType() != events.Envelope_LogMessage {
				d.logger.Debugf("Dump sink (app id %s): Skipping non-log message (type %s)", d.appId, msg.GetEventType().String())
				continue
			}

			d.addMsg(msg)
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(d.inactivityDuration)
		case <-timer.C:
			return
		}
	}
}

func (d *DumpSink) addMsg(msg *events.Envelope) {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.messageRing = d.messageRing.Next()
	d.messageRing.Value = msg
}

func (d *DumpSink) Dump() []*events.Envelope {
	d.lock.RLock()
	defer d.lock.RUnlock()

	data := make([]*events.Envelope, 0, d.messageRing.Len())
	d.messageRing.Next().Do(func(value interface{}) {
		if value == nil {
			return
		}

		msg := value.(*events.Envelope)
		data = append(data, msg)
	})

	return data
}

func (d *DumpSink) AppID() string {
	return d.appId
}

func (d *DumpSink) Logger() *gosteno.Logger {
	return d.logger
}

func (d *DumpSink) Identifier() string {
	return d.appId
}

func (d *DumpSink) ShouldReceiveErrors() bool {
	return true
}
