package sinks

import (
	"sync/atomic"
	"time"

	"github.com/cloudfoundry/dropsonde/metrics"
)

type SinkManagerMetrics struct {
	dumpSinks        int32
	websocketSinks   int32
	syslogSinks      int32
	firehoseSinks    int32
	containerMetrics int32
	done             chan struct{}
}

func NewSinkManagerMetrics() *SinkManagerMetrics {
	mgr := &SinkManagerMetrics{
		done: make(chan struct{}),
	}
	ticker := time.NewTicker(time.Second)
	go mgr.run(ticker)
	return mgr
}

func (s *SinkManagerMetrics) run(ticker *time.Ticker) {
	for range ticker.C {
		select {
		case <-s.done:
			return
		default:
		}

		// metric-documentation-v1: (messageRouter.numberOfDumpSinks) Number of Dump Sinks
		metrics.SendValue("messageRouter.numberOfDumpSinks", float64(atomic.LoadInt32(&s.dumpSinks)), "sinks")
		// metric-documentation-v1: (messageRouter.numberOfSyslogSinks) Number of
		// syslog sinks
		metrics.SendValue("messageRouter.numberOfSyslogSinks", float64(atomic.LoadInt32(&s.syslogSinks)), "sinks")
		// metric-documentation-v1: (messageRouter.numberOfFirehoseSinks) Number of
		// firehose sinks
		metrics.SendValue("messageRouter.numberOfFirehoseSinks", float64(atomic.LoadInt32(&s.firehoseSinks)), "sinks")
		// metric-documentation-v1: (messageRouter.numberOfContainerMetricSinks) Number of
		// container metric sinks
		metrics.SendValue("messageRouter.numberOfContainerMetricSinks", float64(atomic.LoadInt32(&s.containerMetrics)), "sinks")
	}
}

func (s *SinkManagerMetrics) Inc(sink Sink) {
	switch sink.(type) {
	case *DumpSink:
		atomic.AddInt32(&s.dumpSinks, 1)
	case *SyslogSink:
		atomic.AddInt32(&s.syslogSinks, 1)
	case *ContainerMetricSink:
		atomic.AddInt32(&s.containerMetrics, 1)
	}
}

func (s *SinkManagerMetrics) Dec(sink Sink) {
	switch sink.(type) {
	case *DumpSink:
		atomic.AddInt32(&s.dumpSinks, -1)
	case *SyslogSink:
		atomic.AddInt32(&s.syslogSinks, -1)
	case *ContainerMetricSink:
		atomic.AddInt32(&s.containerMetrics, -1)
	}
}

func (s *SinkManagerMetrics) IncFirehose() {
	atomic.AddInt32(&s.firehoseSinks, 1)
}

func (s *SinkManagerMetrics) DecFirehose() {
	atomic.AddInt32(&s.firehoseSinks, -1)
}

func (s *SinkManagerMetrics) Stop() {
	close(s.done)
}
