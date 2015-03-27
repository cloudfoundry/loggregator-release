package dump

import (
	"container/ring"
	"doppler/sinks"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
)

type DumpSink struct {
	sync.RWMutex
	appId                     string
	logger                    *gosteno.Logger
	messageRing               *ring.Ring
	inputChan                 chan *events.Envelope
	inactivityDuration        time.Duration
	droppedMessageCount       uint64
	appDrainMetricsWriterChan chan sinks.DrainMetric
}

func NewDumpSink(appId string, bufferSize uint32, givenLogger *gosteno.Logger, inactivityDuration time.Duration, appDrainMetricsWriterChan chan sinks.DrainMetric) *DumpSink {
	dumpSink := &DumpSink{
		appId:                     appId,
		logger:                    givenLogger,
		messageRing:               ring.New(int(bufferSize)),
		inactivityDuration:        inactivityDuration,
		appDrainMetricsWriterChan: appDrainMetricsWriterChan,
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

func (d *DumpSink) UpdateDroppedMessageCount(messageCount uint64) {
	if messageCount == 0 {
		return
	}

	atomic.AddUint64(&d.droppedMessageCount, messageCount)

	metric := sinks.DrainMetric{AppId: d.appId, DrainURL: "dumpSink", DroppedMsgCount: atomic.LoadUint64(&d.droppedMessageCount)}

	select {
	case d.appDrainMetricsWriterChan <- metric:
	default:
	}
}
