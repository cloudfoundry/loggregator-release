package metrics

import (
	"doppler/sinks"
	"doppler/sinks/dump"
	"doppler/sinks/syslog"
	"doppler/sinks/websocket"
	"sync"

	"github.com/cloudfoundry/dropsonde/metrics"
)

type SinkManagerMetrics struct {
	dumpSinks             int
	websocketSinks        int
	syslogSinks           int
	firehoseSinks         int
	sinkDropUpdateChannel <-chan int64
	lock                  sync.RWMutex
}

func NewSinkManagerMetrics(sinkDropUpdateChannel <-chan int64) *SinkManagerMetrics {
	m := SinkManagerMetrics{
		sinkDropUpdateChannel: sinkDropUpdateChannel,
	}

	go func() {
		for delta := range m.sinkDropUpdateChannel {
			metrics.BatchAddCounter("messageRouter.totalDroppedMessages", uint64(delta))
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
		metrics.SendValue("messageRouter.numberOfDumpSinks", float64(sinkManagerMetrics.dumpSinks), "sinks")

	case *syslog.SyslogSink:
		sinkManagerMetrics.syslogSinks++
		metrics.SendValue("messageRouter.numberOfSyslogSinks", float64(sinkManagerMetrics.syslogSinks), "sinks")

	case *websocket.WebsocketSink:
		sinkManagerMetrics.websocketSinks++
		metrics.SendValue("messageRouter.numberOfWebsocketSinks", float64(sinkManagerMetrics.websocketSinks), "sinks")
	}
}

func (sinkManagerMetrics *SinkManagerMetrics) Dec(sink sinks.Sink) {
	sinkManagerMetrics.lock.Lock()
	defer sinkManagerMetrics.lock.Unlock()

	switch sink.(type) {
	case *dump.DumpSink:
		sinkManagerMetrics.dumpSinks--
		metrics.SendValue("messageRouter.numberOfDumpSinks", float64(sinkManagerMetrics.dumpSinks), "sinks")

	case *syslog.SyslogSink:
		sinkManagerMetrics.syslogSinks--
		metrics.SendValue("messageRouter.numberOfSyslogSinks", float64(sinkManagerMetrics.syslogSinks), "sinks")

	case *websocket.WebsocketSink:
		sinkManagerMetrics.websocketSinks--
		metrics.SendValue("messageRouter.numberOfWebsocketSinks", float64(sinkManagerMetrics.websocketSinks), "sinks")
	}
}

func (sinkManagerMetrics *SinkManagerMetrics) IncFirehose() {
	sinkManagerMetrics.lock.Lock()
	defer sinkManagerMetrics.lock.Unlock()
	sinkManagerMetrics.firehoseSinks++
	metrics.SendValue("messageRouter.numberOfFirehoseSinks", float64(sinkManagerMetrics.firehoseSinks), "sinks")
}

func (sinkManagerMetrics *SinkManagerMetrics) DecFirehose() {
	sinkManagerMetrics.lock.Lock()
	defer sinkManagerMetrics.lock.Unlock()
	sinkManagerMetrics.firehoseSinks--
	metrics.SendValue("messageRouter.numberOfFirehoseSinks", float64(sinkManagerMetrics.firehoseSinks), "sinks")
}
