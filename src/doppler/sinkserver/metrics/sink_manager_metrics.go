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
	dumpSinks              int
	websocketSinks         int
	syslogSinks            int
	firehoseSinks          int
	syslogDrainErrorCounts map[string](map[string]int) // appId -> (url -> count)
	appDrainMetrics        []sinks.Metric
	totalDroppedMessages   int64

	sinkDropUpdateChannel <-chan int64

	lock sync.RWMutex
}

func NewSinkManagerMetrics(sinkDropUpdateChannel <-chan int64) *SinkManagerMetrics {
	m := SinkManagerMetrics{
		syslogDrainErrorCounts: make(map[string](map[string]int)),
		sinkDropUpdateChannel:  sinkDropUpdateChannel,
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

func (sinkManagerMetrics *SinkManagerMetrics) ReportSyslogError(appId string, drainUrl string) {
	sinkManagerMetrics.lock.Lock()
	defer sinkManagerMetrics.lock.Unlock()

	errorsByDrainUrl, ok := sinkManagerMetrics.syslogDrainErrorCounts[appId]

	if !ok {
		errorsByDrainUrl = make(map[string]int)
		sinkManagerMetrics.syslogDrainErrorCounts[appId] = errorsByDrainUrl
	}

	errorsByDrainUrl[drainUrl] = errorsByDrainUrl[drainUrl] + 1
}

func (sinkManagerMetrics *SinkManagerMetrics) AddAppDrainMetrics(metrics []sinks.Metric) {
	sinkManagerMetrics.lock.Lock()
	defer sinkManagerMetrics.lock.Unlock()
	sinkManagerMetrics.appDrainMetrics = metrics
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

	for appId, errorsByUrl := range sinkManagerMetrics.syslogDrainErrorCounts {
		for drainUrl, count := range errorsByUrl {
			data = append(data, instrumentation.Metric{Name: "numberOfSyslogDrainErrors", Value: count, Tags: map[string]interface{}{"appId": appId, "drainUrl": drainUrl}})
		}
	}

	data = append(data, instrumentation.Metric{Name: "totalDroppedMessages", Value: sinkManagerMetrics.totalDroppedMessages})

	for _, metric := range sinkManagerMetrics.appDrainMetrics {
		data = append(data, instrumentation.Metric{
			Name:  metric.Name,
			Value: metric.Value,
			Tags:  metric.Tags,
		})
	}

	return instrumentation.Context{
		Name:    "messageRouter",
		Metrics: data,
	}
}
