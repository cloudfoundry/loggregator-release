package batch

import (
	"fmt"
	"metron/config"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudfoundry/dropsonde/envelope_extensions"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

var metSourceType = proto.String("MET")

//go:generate hel --type BatchCounterIncrementer --output mock_batch_counter_incrementer_test.go

type BatchCounterIncrementer interface {
	BatchIncrementCounter(name string)
}

type DroppedCounter struct {
	conf                 *config.Config
	origin               string
	ip                   string
	deltaDropped         int64
	totalDropped         int64
	writer               BatchChainByteWriter
	incrementer          BatchCounterIncrementer
	timer                *time.Timer
	timerResetLock       sync.Mutex
	congestedDopplerLock sync.Mutex
	congestedDopplers    []string
}

func NewDroppedCounter(byteWriter BatchChainByteWriter, incrementer BatchCounterIncrementer, origin, ip string, conf *config.Config) *DroppedCounter {
	counter := &DroppedCounter{
		origin:      origin,
		ip:          ip,
		conf:        conf,
		writer:      byteWriter,
		incrementer: incrementer,
		timer:       time.NewTimer(time.Second),
	}
	counter.timer.Stop()
	go counter.run()
	return counter
}

func (d *DroppedCounter) Drop(n uint32) {
	defer func() {
		d.timerResetLock.Lock()
		defer d.timerResetLock.Unlock()
		d.timer.Reset(time.Millisecond)
	}()

	atomic.AddInt64(&d.deltaDropped, int64(n))
	atomic.AddInt64(&d.totalDropped, int64(n))
}

func (d *DroppedCounter) DropCongested(n uint32, congestedDoppler string) {
	if n == 0 {
		return
	}
	d.congestedDopplerLock.Lock()
	defer d.congestedDopplerLock.Unlock()
	d.Drop(n)
	d.congestedDopplers = append(d.congestedDopplers, congestedDoppler)
}

func (d *DroppedCounter) Dropped() int64 {
	return atomic.LoadInt64(&d.deltaDropped)
}

func (d *DroppedCounter) run() {
	for range d.timer.C {
		d.sendDroppedMessages()
	}
}

func (d *DroppedCounter) sendDroppedMessages() {
	droppedCount := d.Dropped()
	if droppedCount == 0 {
		return
	}

	totalDropped := atomic.LoadInt64(&d.totalDropped)

	bytes := append(d.encodeCounterEvent(droppedCount, totalDropped), d.encodeLogMessage(droppedCount)...)
	if _, err := d.writer.Write(bytes); err != nil {
		d.incrementer.BatchIncrementCounter("droppedCounter.sendErrors")
		d.timerResetLock.Lock()
		defer d.timerResetLock.Unlock()
		d.timer.Reset(time.Millisecond)
		return
	}

	atomic.AddInt64(&d.deltaDropped, -droppedCount)
}

func (d *DroppedCounter) encodeCounterEvent(droppedCount, totalDropped int64) []byte {
	message := &events.Envelope{
		Origin:     proto.String(d.origin),
		Timestamp:  proto.Int64(time.Now().UnixNano()),
		Ip:         proto.String(d.ip),
		Deployment: proto.String(d.conf.Deployment),
		Index:      proto.String(d.conf.Index),
		Job:        proto.String(d.conf.Job),
		EventType:  events.Envelope_CounterEvent.Enum(),
		CounterEvent: &events.CounterEvent{
			Name:  proto.String("DroppedCounter.droppedMessageCount"),
			Delta: proto.Uint64(uint64(droppedCount)),
			Total: proto.Uint64(uint64(totalDropped)),
		},
	}

	bytes, err := message.Marshal()
	if err != nil {
		d.incrementer.BatchIncrementCounter("droppedCounter.sendErrors")
		d.timer.Reset(time.Millisecond)
		return nil
	}

	return prefixMessage(bytes)
}

func (d *DroppedCounter) encodeLogMessage(droppedCount int64) []byte {
	now := time.Now()
	var dopplerIPs []string

	d.congestedDopplerLock.Lock()
	dopplerIPs = d.congestedDopplers
	d.congestedDopplerLock.Unlock()

	logContent := fmt.Sprintf("Dropped %d message(s) from MetronAgent to Doppler", droppedCount)
	if len(dopplerIPs) != 0 {
		logContent = fmt.Sprintf("Dropped %d message(s) from MetronAgent to Doppler.  Congested dopplers: %s", droppedCount, strings.Join(dopplerIPs, ", "))
		d.clearCongestedDopplers()
	}

	message := &events.Envelope{
		Origin:     proto.String(d.origin),
		Timestamp:  proto.Int64(now.UnixNano()),
		Ip:         proto.String(d.ip),
		Deployment: proto.String(d.conf.Deployment),
		Index:      proto.String(d.conf.Index),
		Job:        proto.String(d.conf.Job),
		EventType:  events.Envelope_LogMessage.Enum(),
		LogMessage: &events.LogMessage{
			MessageType: events.LogMessage_ERR.Enum(),
			Timestamp:   proto.Int64(now.UnixNano()),
			AppId:       proto.String(envelope_extensions.SystemAppId),
			Message:     []byte(logContent),
			SourceType:  metSourceType,
		},
	}

	bytes, err := message.Marshal()
	if err != nil {
		d.incrementer.BatchIncrementCounter("droppedCounter.sendErrors")
		d.timer.Reset(time.Millisecond)
		return nil
	}

	return prefixMessage(bytes)
}

func (d *DroppedCounter) clearCongestedDopplers() {
	d.congestedDopplerLock.Lock()
	d.congestedDopplerLock.Unlock()
	d.congestedDopplers = d.congestedDopplers[:0]
}
