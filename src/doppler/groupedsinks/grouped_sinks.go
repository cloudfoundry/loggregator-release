package groupedsinks

import (
	"doppler/groupedsinks/firehose_group"
	"doppler/groupedsinks/sink_wrapper"
	"doppler/sinks"
	"doppler/sinks/containermetric"
	"doppler/sinks/dump"
	"doppler/sinks/syslog"
	"doppler/sinks/websocket"
	"sync"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
)

func NewGroupedSinks(logger *gosteno.Logger) *GroupedSinks {
	return &GroupedSinks{
		logger:    logger,
		apps:      make(map[string]*AppGroup),
		firehoses: make(map[string]firehose_group.FirehoseGroup),
	}
}

type GroupedSinks struct {
	logger    *gosteno.Logger
	apps      map[string]*AppGroup
	firehoses map[string]firehose_group.FirehoseGroup
	sync.RWMutex
}

func (group *GroupedSinks) RegisterAppSink(in chan<- *events.Envelope, sink sinks.Sink) bool {
	group.Lock()
	defer group.Unlock()

	appId := sink.AppID()
	if appId == "" || sink.Identifier() == "" {
		return false
	}

	sinksForApp, ok := group.apps[appId]
	if !ok || sinksForApp == nil {
		sinksForApp = NewAppGroup()
		group.apps[appId] = sinksForApp
	}
	return sinksForApp.AddSink(sink, in)
}

func (group *GroupedSinks) RegisterFirehoseSink(in chan<- *events.Envelope, sink sinks.Sink) bool {
	group.Lock()
	defer group.Unlock()

	subscriptionId := sink.AppID()
	if subscriptionId == "" {
		return false
	}

	fgroup, ok := group.firehoses[subscriptionId]
	if !ok || fgroup == nil {
		fgroup = firehose_group.NewFirehoseGroup()
		group.firehoses[subscriptionId] = fgroup
	}

	return fgroup.AddSink(sink, in)
}

func (group *GroupedSinks) IsFirehoseRegistered(sink sinks.Sink) bool {
	group.RLock()
	defer group.RUnlock()

	subscriptionId := sink.AppID()
	if subscriptionId == "" {
		return false
	}

	fgroup, ok := group.firehoses[subscriptionId]
	if !ok || fgroup == nil {
		return false
	}

	return fgroup.Exists(sink)
}

func (group *GroupedSinks) Broadcast(appId string, msg *events.Envelope) {
	group.RLock()
	defer group.RUnlock()

	sinksForApp, ok := group.apps[appId]
	if ok && sinksForApp != nil {
		sinksForApp.BroadcastMessage(msg)
	}
	group.broadcastMessageToFirehoses(msg)
}

func (group *GroupedSinks) BroadcastError(appId string, msg *events.Envelope) {
	group.RLock()
	defer group.RUnlock()

	sinksForApp, ok := group.apps[appId]
	if ok && sinksForApp != nil {
		sinksForApp.BroadcastError(msg)
	}
	group.broadcastMessageToFirehoses(msg)
}

func (group *GroupedSinks) broadcastMessageToFirehoses(msg *events.Envelope) {
	for _, fgroup := range group.firehoses {
		if fgroup == nil {
			continue
		}
		fgroup.BroadcastMessage(msg)
	}
}

func (group *GroupedSinks) CountFor(appId string) int {
	group.RLock()
	defer group.RUnlock()

	sinksForApp, ok := group.apps[appId]
	if !ok || sinksForApp == nil {
		return 0
	}
	return sinksForApp.length()
}

func (group *GroupedSinks) DrainFor(appId, drainUrl string) sinks.Sink {
	group.RLock()
	defer group.RUnlock()

	sinksForApp, ok := group.apps[appId]
	if !ok || sinksForApp == nil {
		return nil
	}
	return sinksForApp.Sink(drainUrl)
}

func (group *GroupedSinks) DrainsFor(appId string) []sinks.Sink {
	group.RLock()
	defer group.RUnlock()

	sinksForApp, ok := group.apps[appId]
	if !ok || sinksForApp == nil {
		return nil
	}
	return sinksForApp.SyslogSinks()
}

func (group *GroupedSinks) DumpFor(appId string) *dump.DumpSink {
	group.RLock()
	defer group.RUnlock()

	sinksForApp, ok := group.apps[appId]
	if !ok || sinksForApp == nil {
		return nil
	}
	return sinksForApp.RecentLogsSink(appId)
}

func (group *GroupedSinks) ContainerMetricsFor(appId string) *containermetric.ContainerMetricSink {
	group.RLock()
	defer group.RUnlock()

	sinksForApp, ok := group.apps[appId]
	if !ok || sinksForApp == nil {
		return nil
	}
	return sinksForApp.ContainerMetricsSink("container-metrics-" + appId)
}

func (group *GroupedSinks) WebsocketSinksFor(appId string) []websocket.WebsocketSink {
	group.RLock()
	defer group.RUnlock()

	sinksForApp, ok := group.apps[appId]
	if !ok || sinksForApp == nil {
		return nil
	}
	return sinksForApp.WebsocketSinks()
}

func (group *GroupedSinks) CloseAndDelete(sink sinks.Sink) bool {
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

func (group *GroupedSinks) CloseAndDeleteFirehose(sink sinks.Sink) bool {
	group.Lock()
	defer group.Unlock()

	firehoseSubscriptionId := sink.AppID()

	fgroup, ok := group.firehoses[firehoseSubscriptionId]
	if !ok || fgroup == nil {
		return false
	}

	removed := fgroup.RemoveSink(sink)

	if fgroup.IsEmpty() {
		delete(group.firehoses, firehoseSubscriptionId)
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
	for subscriptionId, fgroup := range group.firehoses {
		if fgroup != nil {
			fgroup.RemoveAllSinks()
		}
		delete(group.firehoses, subscriptionId)
	}
}

type AppGroup struct {
	mu       sync.RWMutex
	wrappers map[string]*sink_wrapper.SinkWrapper
}

func NewAppGroup() *AppGroup {
	return &AppGroup{
		wrappers: make(map[string]*sink_wrapper.SinkWrapper),
	}
}

func (g *AppGroup) AddSink(sink sinks.Sink, in chan<- *events.Envelope) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.exists(sink) {
		return false
	}

	g.wrappers[sink.Identifier()] = &sink_wrapper.SinkWrapper{
		InputChan: in,
		Sink:      sink,
	}

	return true
}

func (g *AppGroup) Exists(sink sinks.Sink) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.exists(sink)
}

// exists needs to be called with read or write lock held.
func (g *AppGroup) exists(sink sinks.Sink) bool {
	w, ok := g.wrappers[sink.Identifier()]
	return ok && w != nil
}

func (g *AppGroup) Sink(id string) sinks.Sink {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.sink(id)
}

func (g *AppGroup) RecentLogsSink(id string) *dump.DumpSink {
	g.mu.RLock()
	defer g.mu.RUnlock()

	dump, ok := g.sink(id).(*dump.DumpSink)
	if !ok {
		return nil
	}
	return dump
}

func (g *AppGroup) ContainerMetricsSink(id string) *containermetric.ContainerMetricSink {
	g.mu.RLock()
	defer g.mu.RUnlock()

	containerMetrics, ok := g.sink(id).(*containermetric.ContainerMetricSink)
	if !ok {
		return nil
	}
	return containerMetrics
}

// sink needs to be called with read or write lock held.
func (g *AppGroup) sink(id string) sinks.Sink {
	wrapper, ok := g.wrappers[id]
	if !ok || wrapper == nil {
		return nil
	}
	return wrapper.Sink
}

func (g *AppGroup) SyslogSinks() []sinks.Sink {
	g.mu.RLock()
	defer g.mu.RUnlock()

	results := []sinks.Sink{}
	for _, wrapper := range g.wrappers {
		if wrapper == nil {
			continue
		}
		_, ok := wrapper.Sink.(*syslog.SyslogSink)
		if !ok {
			continue
		}
		results = append(results, wrapper.Sink)
	}

	return results
}

func (g *AppGroup) WebsocketSinks() []websocket.WebsocketSink {
	g.mu.RLock()
	defer g.mu.RUnlock()

	results := []websocket.WebsocketSink{}
	for _, wrapper := range g.wrappers {
		if wrapper == nil {
			continue
		}
		sink, ok := wrapper.Sink.(*websocket.WebsocketSink)
		if ok {
			results = append(results, *sink)
		}
	}

	return results
}

func (g *AppGroup) RemoveSink(sink sinks.Sink) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.removeSink(sink)
}

func (g *AppGroup) RemoveAllSinks() {
	g.mu.Lock()
	defer g.mu.Unlock()

	for _, wrapper := range g.wrappers {
		if wrapper == nil {
			continue
		}
		g.removeSink(wrapper.Sink)
	}
}

// removeSink needs to be called with write lock held.
func (g *AppGroup) removeSink(sink sinks.Sink) bool {
	wrapper, ok := g.wrappers[sink.Identifier()]
	delete(g.wrappers, sink.Identifier())

	if !ok || wrapper == nil {
		return false
	}

	close(wrapper.InputChan)
	return true
}

func (g *AppGroup) IsEmpty() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.wrappers) == 0
}

func (g *AppGroup) BroadcastMessage(msg *events.Envelope) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, wrapper := range g.wrappers {
		if wrapper == nil {
			continue
		}
		select {
		case wrapper.InputChan <- msg:
		default:
			// TODO: emit metric when this occurs
		}
	}
}

func (g *AppGroup) BroadcastError(msg *events.Envelope) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, wrapper := range g.wrappers {
		if wrapper == nil {
			continue
		}
		if wrapper.Sink.ShouldReceiveErrors() {
			select {
			case wrapper.InputChan <- msg:
			default:
				// TODO: emit metric when this occurs
			}
		}
	}
}

func (g *AppGroup) length() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.wrappers)
}
