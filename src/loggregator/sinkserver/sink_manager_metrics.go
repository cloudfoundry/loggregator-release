package sinkserver

import (
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"loggregator/sinks"
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
	case *sinks.DumpSink:
		sinkManagerMetrics.DumpSinks++
	case *sinks.SyslogSink:
		sinkManagerMetrics.SyslogSinks++
	case *sinks.WebsocketSink:
		sinkManagerMetrics.WebsocketSinks++
	}
}

func (sinkManagerMetrics *SinkManagerMetrics) Dec(sink sinks.Sink) {
	sinkManagerMetrics.Lock()
	defer sinkManagerMetrics.Unlock()

	switch sink.(type) {
	case *sinks.DumpSink:
		sinkManagerMetrics.DumpSinks--
	case *sinks.SyslogSink:
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
