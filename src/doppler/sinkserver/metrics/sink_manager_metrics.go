package metrics

import (
	"doppler/sinks"
	"doppler/sinks/dump"
	"doppler/sinks/syslog"
	"doppler/sinks/websocket"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"sync"
)

type SinkManagerMetrics struct {
	DumpSinks              int
	WebsocketSinks         int
	SyslogSinks            int
	FirehoseSinks          int
	SyslogDrainErrorCounts map[string](map[string]int) // appId -> (url -> count)
	AppDrainMetrics        []sinks.Metric

	sync.RWMutex
}

func NewSinkManagerMetrics() *SinkManagerMetrics {
	return &SinkManagerMetrics{
		SyslogDrainErrorCounts: make(map[string](map[string]int)),
	}
}

func (sinkManagerMetrics *SinkManagerMetrics) Inc(sink sinks.Sink) {
	sinkManagerMetrics.Lock()
	defer sinkManagerMetrics.Unlock()

	switch sink.(type) {
	case *dump.DumpSink:
		sinkManagerMetrics.DumpSinks++
	case *syslog.SyslogSink:
		sinkManagerMetrics.SyslogSinks++
	case *websocket.WebsocketSink:
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
	case *websocket.WebsocketSink:
		sinkManagerMetrics.WebsocketSinks--
	}
}

func (sinkManagerMetrics *SinkManagerMetrics) IncFirehose() {
	sinkManagerMetrics.Lock()
	defer sinkManagerMetrics.Unlock()
	sinkManagerMetrics.FirehoseSinks++
}

func (sinkManagerMetrics *SinkManagerMetrics) DecFirehose() {
	sinkManagerMetrics.Lock()
	defer sinkManagerMetrics.Unlock()
	sinkManagerMetrics.FirehoseSinks--
}

func (sinkManagerMetrics *SinkManagerMetrics) ReportSyslogError(appId string, drainUrl string) {
	sinkManagerMetrics.Lock()
	defer sinkManagerMetrics.Unlock()

	errorsByDrainUrl, ok := sinkManagerMetrics.SyslogDrainErrorCounts[appId]

	if !ok {
		errorsByDrainUrl = make(map[string]int)
		sinkManagerMetrics.SyslogDrainErrorCounts[appId] = errorsByDrainUrl
	}

	errorsByDrainUrl[drainUrl] = errorsByDrainUrl[drainUrl] + 1
}

func (sinkManagerMetrics *SinkManagerMetrics) AddAppDrainMetrics(metrics []sinks.Metric) {
	sinkManagerMetrics.AppDrainMetrics = metrics
}

func (sinkManagerMetrics *SinkManagerMetrics) Emit() instrumentation.Context {
	sinkManagerMetrics.RLock()
	defer sinkManagerMetrics.RUnlock()

	data := []instrumentation.Metric{
		instrumentation.Metric{Name: "numberOfDumpSinks", Value: sinkManagerMetrics.DumpSinks},
		instrumentation.Metric{Name: "numberOfSyslogSinks", Value: sinkManagerMetrics.SyslogSinks},
		instrumentation.Metric{Name: "numberOfWebsocketSinks", Value: sinkManagerMetrics.WebsocketSinks},
		instrumentation.Metric{Name: "numberOfFirehoseSinks", Value: sinkManagerMetrics.FirehoseSinks},
	}

	for appId, errorsByUrl := range sinkManagerMetrics.SyslogDrainErrorCounts {
		for drainUrl, count := range errorsByUrl {
			data = append(data, instrumentation.Metric{Name: "numberOfSyslogDrainErrors", Value: count, Tags: map[string]interface{}{"appId": appId, "drainUrl": drainUrl}})
		}
	}

	data = append(data, instrumentation.Metric{Name: "totalDroppedMessages", Value: sinkManagerMetrics.GetTotalDroppedMessageCount()})

	for _, metric := range sinkManagerMetrics.AppDrainMetrics {
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

func (SinkManagerMetrics *SinkManagerMetrics) GetTotalDroppedMessageCount() int64 {
	SinkManagerMetrics.RLock()
	defer SinkManagerMetrics.RUnlock()

	var total int64
	for _, metric := range SinkManagerMetrics.AppDrainMetrics {
		total += metric.Value
	}

	return total
}
