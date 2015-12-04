package metrics

import (
	"doppler/sinks"
	"doppler/sinks/dump"
	"doppler/sinks/syslog"
	"doppler/sinks/websocket"

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
}

func NewSinkManagerMetrics() *SinkManagerMetrics {
	return &SinkManagerMetrics{}
}

func (s *SinkManagerMetrics) UpdateDroppedMessageCount(delta int64) {
	metrics.BatchAddCounter("messageRouter.totalDroppedMessages", uint64(delta))
}

func (s *SinkManagerMetrics) Inc(sink sinks.Sink) {
	switch sink.(type) {
	case *dump.DumpSink:
		dumpSinks := atomic.AddInt32(&s.dumpSinks, 1)
		metrics.SendValue("messageRouter.numberOfDumpSinks", float64(dumpSinks), "sinks")

	case *syslog.SyslogSink:
		syslogSinks := atomic.AddInt32(&s.syslogSinks, 1)
		metrics.SendValue("messageRouter.numberOfSyslogSinks", float64(syslogSinks), "sinks")

	case *websocket.WebsocketSink:
		websocketSinks := atomic.AddInt32(&s.websocketSinks, 1)
		metrics.SendValue("messageRouter.numberOfWebsocketSinks", float64(websocketSinks), "sinks")

	case *containermetric.ContainerMetricSink:
		containerMetricSinks := atomic.AddInt32(&s.containerMetrics, 1)
		metrics.SendValue("messageRouter.numberOfContainerMetricSinks", float64(containerMetricSinks), "sinks")
	}
}

func (s *SinkManagerMetrics) Dec(sink sinks.Sink) {
	switch sink.(type) {
	case *dump.DumpSink:
		dumpSinks := atomic.AddInt32(&s.dumpSinks, -1)
		metrics.SendValue("messageRouter.numberOfDumpSinks", float64(dumpSinks), "sinks")

	case *syslog.SyslogSink:
		syslogSinks := atomic.AddInt32(&s.syslogSinks, -1)
		metrics.SendValue("messageRouter.numberOfSyslogSinks", float64(syslogSinks), "sinks")

	case *websocket.WebsocketSink:
		websocketSinks := atomic.AddInt32(&s.websocketSinks, -1)
		metrics.SendValue("messageRouter.numberOfWebsocketSinks", float64(websocketSinks), "sinks")

	case *containermetric.ContainerMetricSink:
		containerMetricSinks := atomic.AddInt32(&s.containerMetrics, -1)
		metrics.SendValue("messageRouter.numberOfContainerMetricSinks", float64(containerMetricSinks), "sinks")
	}
}

func (s *SinkManagerMetrics) IncFirehose() {
	firehoseSinks := atomic.AddInt32(&s.firehoseSinks, 1)
	metrics.SendValue("messageRouter.numberOfFirehoseSinks", float64(firehoseSinks), "sinks")
}

func (s *SinkManagerMetrics) DecFirehose() {
	firehoseSinks := atomic.AddInt32(&s.firehoseSinks, -1)
	metrics.SendValue("messageRouter.numberOfFirehoseSinks", float64(firehoseSinks), "sinks")
}
