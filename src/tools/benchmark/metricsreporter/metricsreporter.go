package metricsreporter

import (
	"fmt"
	"time"

	"io"
	"sync/atomic"
)

type MetricsReporter struct {
	reportTime time.Duration
	stopChan   chan struct{}
	writer     io.Writer
	sent       int32
	received   int32
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
			// emit the metric for set reportTime, then reset values
			loss := (float32(r.sent-r.received) / float32(r.sent)) * 100
			fmt.Fprintf(r.writer, "%v, %v, %v\n", r.sent, r.received, loss)
			r.reset()
		case <-r.stopChan:
			return
		}
	}
}

func (r *MetricsReporter) Stop() {
	close(r.stopChan)
}

func (r *MetricsReporter) IncrementSentMessages() {
	atomic.AddInt32(&r.sent, 1)
}

func (r *MetricsReporter) IncrementReceivedMessages() {
	atomic.AddInt32(&r.received, 1)
}

func (r *MetricsReporter) reset() {
	r.sent = 0
	r.received = 0
}
