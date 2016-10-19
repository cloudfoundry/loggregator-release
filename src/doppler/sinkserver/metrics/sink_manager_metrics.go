package metrics

import (
	"doppler/sinks"
	"doppler/sinks/dump"
	"doppler/sinks/syslog"
	"doppler/sinks/websocket"
	"time"

	"doppler/sinks/containermetric"
	"sync/atomic"

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

		metrics.SendValue("messageRouter.numberOfDumpSinks", float64(atomic.LoadInt32(&s.dumpSinks)), "sinks")
		metrics.SendValue("messageRouter.numberOfWebsocketSinks", float64(atomic.LoadInt32(&s.websocketSinks)), "sinks")
		metrics.SendValue("messageRouter.numberOfSyslogSinks", float64(atomic.LoadInt32(&s.syslogSinks)), "sinks")
		metrics.SendValue("messageRouter.numberOfFirehoseSinks", float64(atomic.LoadInt32(&s.firehoseSinks)), "sinks")
		metrics.SendValue("messageRouter.numberOfContainerMetricSinks", float64(atomic.LoadInt32(&s.containerMetrics)), "sinks")
	}
}

func (s *SinkManagerMetrics) UpdateDroppedMessageCount(delta int64) {
	metrics.BatchAddCounter("messageRouter.totalDroppedMessages", uint64(delta))
}

func (s *SinkManagerMetrics) Inc(sink sinks.Sink) {
	switch sink.(type) {
	case *dump.DumpSink:
		atomic.AddInt32(&s.dumpSinks, 1)
	case *syslog.SyslogSink:
		atomic.AddInt32(&s.syslogSinks, 1)
	case *websocket.WebsocketSink:
		atomic.AddInt32(&s.websocketSinks, 1)
	case *containermetric.ContainerMetricSink:
		atomic.AddInt32(&s.containerMetrics, 1)
	}
}

func (s *SinkManagerMetrics) Dec(sink sinks.Sink) {
	switch sink.(type) {
	case *dump.DumpSink:
		atomic.AddInt32(&s.dumpSinks, -1)
	case *syslog.SyslogSink:
		atomic.AddInt32(&s.syslogSinks, -1)
	case *websocket.WebsocketSink:
		atomic.AddInt32(&s.websocketSinks, -1)
	case *containermetric.ContainerMetricSink:
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
