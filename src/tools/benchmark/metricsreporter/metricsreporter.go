package metricsreporter

import (
	"fmt"
	"time"

	"io"
	"sync"
	"sync/atomic"
)

type MetricsReporter struct {
	reportTime    time.Duration
	stopChan      chan struct{}
	writer        io.Writer
	sent          int32
	received      int32
	totalSent     int32
	totalReceived int32
	numTicks      int32
	lock          sync.Mutex
}

func New(reportTime time.Duration, writer io.Writer) *MetricsReporter {
	return &MetricsReporter{
		reportTime: reportTime,
		writer:     writer,
		stopChan:   make(chan struct{}),
	}
}

func (r *MetricsReporter) Start() {
	ticker := time.NewTicker(r.reportTime)
	fmt.Fprintf(r.writer, "Sent, Received, PercentLoss\n")
	for {
		select {
		case <-ticker.C:
			sent := atomic.LoadInt32(&r.sent)
			received := atomic.LoadInt32(&r.received)
			// emit the metric for set reportTime, then reset values
			loss := (float32(sent-received) / float32(sent)) * 100
			fmt.Fprintf(r.writer, "%v, %v, %v%%\n", sent, received, loss)
			r.reset()
		case <-r.stopChan:
			return
		}
	}
}

func (r *MetricsReporter) Stop() {
	r.lock.Lock()
	defer r.lock.Unlock()

	close(r.stopChan)
	loss := (float32(r.totalSent-r.totalReceived) / float32(r.totalSent)) * 100
	averageSent := float64(r.totalSent) / float64(r.numTicks)
	averageReceived := float64(r.totalReceived) / float64(r.numTicks)
	fmt.Fprintf(r.writer, "Averages: %v, %v, %v%%\n", averageSent, averageReceived, loss)
}

func (r *MetricsReporter) IncrementSentMessages() {
	atomic.AddInt32(&r.sent, 1)
}

func (r *MetricsReporter) getSent() int32 {
	return atomic.LoadInt32(&r.sent)
}

func (r *MetricsReporter) getReceived() int32 {
	return atomic.LoadInt32(&r.received)
}

func (r *MetricsReporter) IncrementReceivedMessages() {
	atomic.AddInt32(&r.received, 1)
}

func (r *MetricsReporter) GetNumTicks() int32 {
	return atomic.LoadInt32(&r.numTicks)
}

func (r *MetricsReporter) reset() {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.totalSent += atomic.LoadInt32(&r.sent)
	r.totalReceived += atomic.LoadInt32(&r.received)

	atomic.StoreInt32(&r.sent, 0)
	atomic.StoreInt32(&r.received, 0)
	atomic.AddInt32(&r.numTicks, 1)
}
