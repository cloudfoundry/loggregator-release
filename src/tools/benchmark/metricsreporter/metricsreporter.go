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
	sentCounter     Counter
	receivedCounter Counter
	numTicks        int32
	lock            sync.Mutex
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
			sent := r.sentCounter.GetValue()
			received := r.receivedCounter.GetValue()
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

	sentTotal := r.sentCounter.GetTotal()
	receivedTotal := r.receivedCounter.GetTotal()

	averageSent := float64(sentTotal) / float64(r.numTicks)
	averageReceived := float64(receivedTotal) / float64(r.numTicks)
	loss := (float32(sentTotal-receivedTotal) / float32(sentTotal)) * 100

	fmt.Fprintf(r.writer, "Averages: %v, %v, %v%%\n", averageSent, averageReceived, loss)
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

func (r *MetricsReporter) reset() {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.sentCounter.Reset()
	r.receivedCounter.Reset()

	atomic.AddInt32(&r.numTicks, 1)
}
