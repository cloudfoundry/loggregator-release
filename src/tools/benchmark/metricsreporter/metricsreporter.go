package metricsreporter

import (
	"fmt"
	"time"

	"io"
	"sync"
	"sync/atomic"
)

type MetricsReporter struct {
	reportTime      time.Duration
	stopChan        chan struct{}
	writer          io.Writer
	counters        []*Counter
	sentCounter     Counter
	receivedCounter Counter
	numTicks        int32
	lock            sync.Mutex
}

func New(reportTime time.Duration, writer io.Writer, counters ...*Counter) *MetricsReporter {
	return &MetricsReporter{
		reportTime: reportTime,
		writer:     writer,
		counters:   counters,
		stopChan:   make(chan struct{}),
	}
}

func (r *MetricsReporter) Start() {
	ticker := time.NewTicker(r.reportTime)
	fmt.Fprintf(r.writer, "Sent, Received, PercentLoss%s\n", r.getCounterNames())
	for {
		select {
		case <-ticker.C:
			sent := r.sentCounter.GetValue()
			received := r.receivedCounter.GetValue()
			// emit the metric for set reportTime, then reset values
			loss := (float32(sent-received) / float32(sent)) * 100
			counterOut := r.getCounterValues()
			fmt.Fprintf(r.writer, "%v, %v, %v%%%s\n", sent, received, loss, counterOut)
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

	sentTotal := r.sentCounter.GetTotal()
	receivedTotal := r.receivedCounter.GetTotal()

	averageSent := float64(sentTotal) / float64(r.numTicks)
	averageReceived := float64(receivedTotal) / float64(r.numTicks)
	loss := (float32(sentTotal-receivedTotal) / float32(sentTotal)) * 100

	counterOut := r.getCounterAverages()

	fmt.Fprintf(r.writer, "Averages: %v, %v, %v%%%s\n", averageSent, averageReceived, loss, counterOut)
}

func (r *MetricsReporter) GetSentCounter() *Counter {
	return &r.sentCounter
}

func (r *MetricsReporter) GetReceivedCounter() *Counter {
	return &r.receivedCounter
}

func (r *MetricsReporter) GetNumTicks() int32 {
	return atomic.LoadInt32(&r.numTicks)
}

func (r *MetricsReporter) getCounterValues() string {
	var values string
	for _, counter := range r.counters {
		values = fmt.Sprintf("%s, %v", values, counter.GetValue())
	}
	return values
}

func (r *MetricsReporter) getCounterNames() string {
	var values string
	for _, counter := range r.counters {
		values = fmt.Sprintf("%s, %v", values, counter.GetName())
	}
	return values
}

func (r *MetricsReporter) getCounterAverages() string {
	var values string
	for _, counter := range r.counters {
		average := float64(counter.GetTotal()) / float64(r.numTicks)
		values = fmt.Sprintf("%s, %v", values, average)
	}
	return values
}

func (r *MetricsReporter) reset() {
	r.lock.Lock()
	defer r.lock.Unlock()

	for _, counter := range r.counters {
		counter.Reset()
	}

	r.sentCounter.Reset()
	r.receivedCounter.Reset()

	atomic.AddInt32(&r.numTicks, 1)
}
