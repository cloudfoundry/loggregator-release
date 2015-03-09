package dump

import (
	"container/ring"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"sync"
	"sync/atomic"
	"time"
)

type DumpSink struct {
	sync.RWMutex
	appId               string
	logger              *gosteno.Logger
	messageRing         *ring.Ring
	inputChan           chan *events.Envelope
	inactivityDuration  time.Duration
	droppedMessageCount int64
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

	d.messageRing = d.messageRing.Next()
	d.messageRing.Value = msg
}

func (d *DumpSink) Dump() []*events.Envelope {
	d.RLock()
	defer d.RUnlock()

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

func (d *DumpSink) GetInstrumentationMetric() instrumentation.Metric {
	count := atomic.LoadInt64(&d.droppedMessageCount)
	if count != 0 {
		return instrumentation.Metric{Name: "numberOfMessagesLost", Tags: map[string]interface{}{"appId": string(d.appId), "drainUrl": "dumpSink"}, Value: count}
	}
	return instrumentation.Metric{}
}

func (d *DumpSink) UpdateDroppedMessageCount(messageCount int64) {
	atomic.AddInt64(&d.droppedMessageCount, messageCount)
}
