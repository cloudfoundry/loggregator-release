package metrics

import (
	"doppler/sinks"
	"doppler/sinks/dump"
	"doppler/sinks/syslog"
	"doppler/sinks/websocket"
	"sync"
	"sync/atomic"

	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
)

type SinkManagerMetrics struct {
	DumpSinks                    uint64
	WebsocketSinks               uint64
	SyslogSinks                  uint64
	FirehoseSinks                uint64
	SyslogDrainErrorCounts       SyslogDrainErrorCounts
	AppDrainMetrics              map[AppDrainMetricKey]uint64
	AppDrainMetricsReceiverChan  chan sinks.DrainMetric
	TotalAppDrainMessagesDropped uint64

	sync.RWMutex
}

type AppDrainMetricKey struct {
	AppId    string
	DrainURL string
}

type SyslogDrainErrorCounts struct {
	ErrorCountMap map[string](map[string]int) // appId -> (url -> count)
	sync.RWMutex
}

func NewSinkManagerMetrics() *SinkManagerMetrics {
	return &SinkManagerMetrics{
		SyslogDrainErrorCounts:      SyslogDrainErrorCounts{ErrorCountMap: make(map[string](map[string]int))},
		AppDrainMetrics:             make(map[AppDrainMetricKey]uint64),
		AppDrainMetricsReceiverChan: make(chan sinks.DrainMetric, 128),
	}
}

func (sinkManagerMetrics *SinkManagerMetrics) ListenForUpdatedAppDrainMetrics() {

	for drainMetric := range sinkManagerMetrics.AppDrainMetricsReceiverChan {
		key := AppDrainMetricKey{AppId: drainMetric.AppId, DrainURL: drainMetric.DrainURL}
		sinkManagerMetrics.Lock()
		previousDroppedMsgCount := sinkManagerMetrics.AppDrainMetrics[key]
		sinkManagerMetrics.AppDrainMetrics[key] = drainMetric.DroppedMsgCount
		sinkManagerMetrics.Unlock()
		delta := drainMetric.DroppedMsgCount - previousDroppedMsgCount
		atomic.AddUint64(&sinkManagerMetrics.TotalAppDrainMessagesDropped, delta)
	}
}

func (sinkManagerMetrics *SinkManagerMetrics) AddAppDrainEntry(appId string, drainURL string) {
	key := AppDrainMetricKey{AppId: appId, DrainURL: drainURL}
	sinkManagerMetrics.Lock()
	sinkManagerMetrics.AppDrainMetrics[key] = 0
	sinkManagerMetrics.Unlock()
}

func (sinkManagerMetrics *SinkManagerMetrics) RemoveAppDrainEntry(appId string, drainURL string) {
	key := AppDrainMetricKey{AppId: appId, DrainURL: drainURL}
	sinkManagerMetrics.Lock()
	delete(sinkManagerMetrics.AppDrainMetrics, key)
	sinkManagerMetrics.Unlock()
}

func (sinkManagerMetrics *SinkManagerMetrics) Inc(sink sinks.Sink) {

	switch sink.(type) {
	case *dump.DumpSink:
		atomic.AddUint64(&sinkManagerMetrics.DumpSinks, 1)
	case *syslog.SyslogSink:
		atomic.AddUint64(&sinkManagerMetrics.SyslogSinks, 1)
	case *websocket.WebsocketSink:
		atomic.AddUint64(&sinkManagerMetrics.WebsocketSinks, 1)
	}
}

func (sinkManagerMetrics *SinkManagerMetrics) Dec(sink sinks.Sink) {

	switch sink.(type) {
	case *dump.DumpSink:
		atomic.AddUint64(&sinkManagerMetrics.DumpSinks, ^uint64(0))
	case *syslog.SyslogSink:
		atomic.AddUint64(&sinkManagerMetrics.SyslogSinks, ^uint64(0))
	case *websocket.WebsocketSink:
		atomic.AddUint64(&sinkManagerMetrics.WebsocketSinks, ^uint64(0))
	}
}

func (sinkManagerMetrics *SinkManagerMetrics) IncFirehose() {
	atomic.AddUint64(&sinkManagerMetrics.FirehoseSinks, 1)
}

func (sinkManagerMetrics *SinkManagerMetrics) DecFirehose() {
	atomic.AddUint64(&sinkManagerMetrics.FirehoseSinks, ^uint64(0))
}

func (sinkManagerMetrics *SinkManagerMetrics) ReportSyslogError(appId string, drainUrl string) {
	sinkManagerMetrics.SyslogDrainErrorCounts.Lock()
	defer sinkManagerMetrics.SyslogDrainErrorCounts.Unlock()

	errorsByDrainUrl, ok := sinkManagerMetrics.SyslogDrainErrorCounts.ErrorCountMap[appId]

	if !ok {
		errorsByDrainUrl = make(map[string]int)
		sinkManagerMetrics.SyslogDrainErrorCounts.ErrorCountMap[appId] = errorsByDrainUrl
	}

	errorsByDrainUrl[drainUrl] = errorsByDrainUrl[drainUrl] + 1
}

func (sinkManagerMetrics *SinkManagerMetrics) CreateAppDrainMetrics() []instrumentation.Metric {
	var instrumentationMetrics []instrumentation.Metric
	for key, value := range sinkManagerMetrics.AppDrainMetrics {
		tags := map[string]interface{}{"appId": key.AppId, "drainUrl": key.DrainURL}
		metric := instrumentation.Metric{Name: "numberOfMessagesLost", Tags: tags, Value: value}
		instrumentationMetrics = append(instrumentationMetrics, metric)
	}
	return instrumentationMetrics
}

func (sinkManagerMetrics *SinkManagerMetrics) Emit() instrumentation.Context {

	data := []instrumentation.Metric{
		instrumentation.Metric{Name: "numberOfDumpSinks", Value: atomic.LoadUint64(&sinkManagerMetrics.DumpSinks)},
		instrumentation.Metric{Name: "numberOfSyslogSinks", Value: atomic.LoadUint64(&sinkManagerMetrics.SyslogSinks)},
		instrumentation.Metric{Name: "numberOfWebsocketSinks", Value: atomic.LoadUint64(&sinkManagerMetrics.WebsocketSinks)},
		instrumentation.Metric{Name: "numberOfFirehoseSinks", Value: atomic.LoadUint64(&sinkManagerMetrics.FirehoseSinks)},
	}

	sinkManagerMetrics.SyslogDrainErrorCounts.Lock()
	for appId, errorsByUrl := range sinkManagerMetrics.SyslogDrainErrorCounts.ErrorCountMap {
		for drainUrl, count := range errorsByUrl {
			data = append(data, instrumentation.Metric{Name: "numberOfSyslogDrainErrors", Value: count, Tags: map[string]interface{}{"appId": appId, "drainUrl": drainUrl}})
		}
	}
	sinkManagerMetrics.SyslogDrainErrorCounts.Unlock()

	sinkManagerMetrics.RLock()
	data = append(data, instrumentation.Metric{Name: "totalDroppedMessages", Value: atomic.LoadUint64(&sinkManagerMetrics.TotalAppDrainMessagesDropped)})
	data = append(data, sinkManagerMetrics.CreateAppDrainMetrics()...)
	sinkManagerMetrics.RUnlock()

	return instrumentation.Context{
		Name:    "messageRouter",
		Metrics: data,
	}
}
