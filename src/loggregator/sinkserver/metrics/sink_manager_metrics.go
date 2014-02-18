package metrics

import (
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"loggregator/sinks"
	"loggregator/sinks/dump"
	"loggregator/sinks/syslog"
	"sync"
)

type SinkManagerMetrics struct {
	DumpSinks      int
	WebsocketSinks int
	SyslogSinks    int
	sync.RWMutex
}

func NewSinkManagerMetrics() *SinkManagerMetrics {
	return &SinkManagerMetrics{}
}

func (sinkManagerMetrics *SinkManagerMetrics) Inc(sink sinks.Sink) {
	sinkManagerMetrics.Lock()
	defer sinkManagerMetrics.Unlock()

	switch sink.(type) {
	case *dump.DumpSink:
		sinkManagerMetrics.DumpSinks++
	case *syslog.SyslogSink:
		sinkManagerMetrics.SyslogSinks++
	case *sinks.WebsocketSink:
		sinkManagerMetrics.WebsocketSinks++
	}
}

func (sinkManagerMetrics *SinkManagerMetrics) Dec(sink sinks.Sink) {
	sinkManagerMetrics.Lock()
	defer sinkManagerMetrics.Unlock()

	switch sink.(type) {
	case *dump.DumpSink:
		sinkManagerMetrics.DumpSinks--
	case *syslog.SyslogSink:
		sinkManagerMetrics.SyslogSinks--
	case *sinks.WebsocketSink:
		sinkManagerMetrics.WebsocketSinks--
	}
}

func (sinkManagerMetrics *SinkManagerMetrics) Emit() instrumentation.Context {
	sinkManagerMetrics.RLock()
	defer sinkManagerMetrics.RUnlock()

	data := []instrumentation.Metric{
		instrumentation.Metric{Name: "numberOfDumpSinks", Value: sinkManagerMetrics.DumpSinks},
		instrumentation.Metric{Name: "numberOfSyslogSinks", Value: sinkManagerMetrics.SyslogSinks},
		instrumentation.Metric{Name: "numberOfWebsocketSinks", Value: sinkManagerMetrics.WebsocketSinks},
	}

	return instrumentation.Context{
		Name:    "messageRouter",
		Metrics: data,
	}
}
