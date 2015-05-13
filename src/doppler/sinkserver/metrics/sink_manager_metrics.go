package metrics

import (
	"doppler/sinks"
	"doppler/sinks/dump"
	"doppler/sinks/syslog"
	"doppler/sinks/websocket"
	"sync"

	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
)

type SinkManagerMetrics struct {
	dumpSinks             int
	websocketSinks        int
	syslogSinks           int
	firehoseSinks         int
	totalDroppedMessages  int64
	sinkDropUpdateChannel <-chan int64
	lock                  sync.RWMutex
}

func NewSinkManagerMetrics(sinkDropUpdateChannel <-chan int64) *SinkManagerMetrics {
	m := SinkManagerMetrics{
		sinkDropUpdateChannel: sinkDropUpdateChannel,
	}

	go func() {
		for delta := range m.sinkDropUpdateChannel {
			m.lock.Lock()
			m.totalDroppedMessages += delta
			m.lock.Unlock()
		}
	}()

	return &m
}

func (sinkManagerMetrics *SinkManagerMetrics) Inc(sink sinks.Sink) {
	sinkManagerMetrics.lock.Lock()
	defer sinkManagerMetrics.lock.Unlock()

	switch sink.(type) {
	case *dump.DumpSink:
		sinkManagerMetrics.dumpSinks++
	case *syslog.SyslogSink:
		sinkManagerMetrics.syslogSinks++
	case *websocket.WebsocketSink:
		sinkManagerMetrics.websocketSinks++
	}
}

func (sinkManagerMetrics *SinkManagerMetrics) Dec(sink sinks.Sink) {
	sinkManagerMetrics.lock.Lock()
	defer sinkManagerMetrics.lock.Unlock()

	switch sink.(type) {
	case *dump.DumpSink:
		sinkManagerMetrics.dumpSinks--
	case *syslog.SyslogSink:
		sinkManagerMetrics.syslogSinks--
	case *websocket.WebsocketSink:
		sinkManagerMetrics.websocketSinks--
	}
}

func (sinkManagerMetrics *SinkManagerMetrics) IncFirehose() {
	sinkManagerMetrics.lock.Lock()
	defer sinkManagerMetrics.lock.Unlock()
	sinkManagerMetrics.firehoseSinks++
}

func (sinkManagerMetrics *SinkManagerMetrics) DecFirehose() {
	sinkManagerMetrics.lock.Lock()
	defer sinkManagerMetrics.lock.Unlock()
	sinkManagerMetrics.firehoseSinks--
}

func (sinkManagerMetrics *SinkManagerMetrics) Emit() instrumentation.Context {
	sinkManagerMetrics.lock.RLock()
	defer sinkManagerMetrics.lock.RUnlock()

	data := []instrumentation.Metric{
		instrumentation.Metric{Name: "numberOfDumpSinks", Value: sinkManagerMetrics.dumpSinks},
		instrumentation.Metric{Name: "numberOfSyslogSinks", Value: sinkManagerMetrics.syslogSinks},
		instrumentation.Metric{Name: "numberOfWebsocketSinks", Value: sinkManagerMetrics.websocketSinks},
		instrumentation.Metric{Name: "numberOfFirehoseSinks", Value: sinkManagerMetrics.firehoseSinks},
	}

	data = append(data, instrumentation.Metric{Name: "totalDroppedMessages", Value: sinkManagerMetrics.totalDroppedMessages})

	return instrumentation.Context{
		Name:    "messageRouter",
		Metrics: data,
	}
}
