package sinks

import (
	"sync"

	"code.cloudfoundry.org/loggregator/metricemitter"
	"github.com/cloudfoundry/sonde-go/events"
)

// MetricClient creates new Counter and Gauge metrics to be emitted periodically.
type MetricClient interface {
	NewCounter(name string, opts ...metricemitter.MetricOption) *metricemitter.Counter
	NewGauge(name, unit string, opts ...metricemitter.MetricOption) *metricemitter.Gauge
}

type MetricBatcher interface {
	BatchIncrementCounter(name string)
}

func NewGroupedSinks(mc MetricClient) *GroupedSinks {
	droppedMetric := mc.NewCounter("sinks.dropped",
		metricemitter.WithVersion(2, 0),
	)
	errorMetric := mc.NewCounter("sinks.errors.dropped",
		metricemitter.WithVersion(2, 0),
	)

	return &GroupedSinks{
		apps:          make(map[string]*AppGroup),
		droppedMetric: droppedMetric,
		errorMetric:   errorMetric,
	}
}

type GroupedSinks struct {
	sync.RWMutex

	apps          map[string]*AppGroup
	droppedMetric *metricemitter.Counter
	errorMetric   *metricemitter.Counter
}

func (group *GroupedSinks) RegisterAppSink(in chan<- *events.Envelope, sink Sink) bool {
	group.Lock()
	defer group.Unlock()

	appId := sink.AppID()
	if appId == "" || sink.Identifier() == "" {
		return false
	}

	sinksForApp, ok := group.apps[appId]
	if !ok || sinksForApp == nil {
		sinksForApp = NewAppGroup(
			group.droppedMetric,
			group.errorMetric,
		)
		group.apps[appId] = sinksForApp
	}
	return sinksForApp.AddSink(sink, in)
}

func (group *GroupedSinks) Broadcast(appId string, msg *events.Envelope) {
	group.RLock()
	defer group.RUnlock()

	sinksForApp, ok := group.apps[appId]
	if ok && sinksForApp != nil {
		sinksForApp.BroadcastMessage(msg)
	}
}

func (group *GroupedSinks) DumpFor(appId string) *DumpSink {
	group.RLock()
	defer group.RUnlock()

	sinksForApp, ok := group.apps[appId]
	if !ok || sinksForApp == nil {
		return nil
	}
	return sinksForApp.RecentLogsSink(appId)
}

func (group *GroupedSinks) CloseAndDelete(sink Sink) bool {
	group.Lock()
	defer group.Unlock()

	appId := sink.AppID()

	sinksForApp, ok := group.apps[appId]
	if !ok || sinksForApp == nil {
		return false
	}

	removed := sinksForApp.RemoveSink(sink)
	if sinksForApp.IsEmpty() {
		delete(group.apps, appId)
	}

	return removed
}

func (group *GroupedSinks) DeleteAll() {
	group.Lock()
	defer group.Unlock()

	for appId, sinksForApp := range group.apps {
		if sinksForApp != nil {
			sinksForApp.RemoveAllSinks()
		}
		delete(group.apps, appId)
	}
}
