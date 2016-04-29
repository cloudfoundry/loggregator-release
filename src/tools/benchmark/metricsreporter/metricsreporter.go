package metricsreporter

import (
	"fmt"
	"time"

	"io"
	"sync"
	"sync/atomic"
)

type MetricsReporter struct {
	reportInterval  time.Duration
	stop, done      chan struct{}
	writer          io.Writer
	counters        []*Counter
	sentCounter     Counter
	receivedCounter Counter
	numTicks        int32
	lock            sync.Mutex

	timeLock   sync.Mutex
	start, end time.Time
}

func New(reportTime time.Duration, writer io.Writer, counters ...*Counter) *MetricsReporter {
	return &MetricsReporter{
		reportInterval: reportTime,
		writer:         writer,
		counters:       counters,
		stop:           make(chan struct{}),
		done:           make(chan struct{}),
	}
}

func (r *MetricsReporter) Start() {
	ticker := time.NewTicker(r.reportInterval)
	fmt.Fprintf(r.writer, "Runtime, Sent, Received, Rate, PercentLoss%s\n", r.counterNames())

	r.timeLock.Lock()
	r.start = time.Now()
	r.end = r.start
	r.timeLock.Unlock()

	defer func() {
		r.syncEnd()
		close(r.done)
	}()

	for {
		select {
		case <-ticker.C:
			sent := r.sentCounter.GetValue()
			received := r.receivedCounter.GetValue()
			// emit the metric for set reportTime, then reset values
			loss := percentLoss(sent, received)
			counterOut := r.counterValues()
			r.syncEnd()

			fmt.Fprintf(r.writer, "%s, %d, %d, %.2f/s, %.2f%%%s\n", r.Duration(), sent, received, r.Rate(), loss, counterOut)
			r.reset()
		case <-r.stop:
			return
		}
	}
}

func (r *MetricsReporter) Stop() {
	close(r.stop)
	<-r.done

	r.lock.Lock()
	defer r.lock.Unlock()

	sentTotal := r.sentCounter.GetTotal()
	receivedTotal := r.receivedCounter.GetTotal()

	averageSent := float64(sentTotal) / float64(r.numTicks)
	averageReceived := float64(receivedTotal) / float64(r.numTicks)
	loss := percentLoss(sentTotal, receivedTotal)

	counterOut := r.counterAverages()

	fmt.Fprintf(r.writer, "Averages: %.f, %.f, %.f, %.f/s, %.2f%%%s\n", r.Duration().Seconds(), averageSent, averageReceived, r.Rate(), loss, counterOut)
}

func (r *MetricsReporter) Duration() time.Duration {
	r.timeLock.Lock()
	defer r.timeLock.Unlock()
	return r.end.Sub(r.start)
}

func (r *MetricsReporter) Rate() float64 {
	return float64(r.sentCounter.GetTotal()) / r.Duration().Seconds()
}

func (r *MetricsReporter) SentCounter() *Counter {
	return &r.sentCounter
}

func (r *MetricsReporter) ReceivedCounter() *Counter {
	return &r.receivedCounter
}

func (r *MetricsReporter) NumTicks() int32 {
	return atomic.LoadInt32(&r.numTicks)
}

func (r *MetricsReporter) syncEnd() {
	r.timeLock.Lock()
	defer r.timeLock.Unlock()
	r.end = time.Now()
}

func (r *MetricsReporter) counterValues() string {
	var values string
	for _, counter := range r.counters {
		values = fmt.Sprintf("%s, %v", values, counter.GetValue())
	}
	return values
}

func (r *MetricsReporter) counterNames() string {
	var values string
	for _, counter := range r.counters {
		values = fmt.Sprintf("%s, %v", values, counter.GetName())
	}
	return values
}

func (r *MetricsReporter) counterAverages() string {
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

func percentLoss(sent, received uint64) float32 {
	return (float32(sent) - float32(received)) / float32(sent) * 100
}
