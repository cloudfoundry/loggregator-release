package groupedsinks

import (
	"doppler/groupedsinks/firehose_group"
	"doppler/groupedsinks/sink_wrapper"
	"doppler/sinks"
	"doppler/sinks/dump"
	"doppler/sinks/syslog"
	"doppler/sinks/websocket"
	"github.com/cloudfoundry/dropsonde/events"
	"sync"
)

func NewGroupedSinks() *GroupedSinks {
	return &GroupedSinks{
		apps:      make(map[string]map[string]*sink_wrapper.SinkWrapper),
		firehoses: make(map[string]firehose_group.FirehoseGroup),
	}
}

type GroupedSinks struct {
	apps      map[string]map[string]*sink_wrapper.SinkWrapper
	firehoses map[string]firehose_group.FirehoseGroup
	sync.RWMutex
}

func (group *GroupedSinks) Register(in chan<- *events.Envelope, sink sinks.Sink) bool {
	group.Lock()
	defer group.Unlock()

	appId := sink.AppId()
	if appId == "" || sink.Identifier() == "" {
		return false
	}
	sinksForApp := group.apps[appId]
	if sinksForApp == nil {
		group.apps[appId] = make(map[string]*sink_wrapper.SinkWrapper)
		sinksForApp = group.apps[appId]
	}

	if _, ok := sinksForApp[sink.Identifier()]; ok {
		return false
	}
	sinksForApp[sink.Identifier()] = &sink_wrapper.SinkWrapper{InputChan: in, Sink: sink}
	return true
}

func (group *GroupedSinks) RegisterFirehose(in chan<- *events.Envelope, sink sinks.Sink) bool {
	group.Lock()
	defer group.Unlock()

	subscriptionId := sink.AppId()
	if subscriptionId == "" {
		return false
	}

	fgroup := group.firehoses[subscriptionId]
	if fgroup == nil {
		group.firehoses[subscriptionId] = firehose_group.NewFirehoseGroup()
		fgroup = group.firehoses[subscriptionId]
	}

	return fgroup.AddSink(&sink_wrapper.SinkWrapper{InputChan: in, Sink: sink})
}

func (group *GroupedSinks) BroadCast(appId string, msg *events.Envelope) {
	group.RLock()
	defer group.RUnlock()

	for _, wrapper := range group.apps[appId] {
		wrapper.InputChan <- msg
	}

	group.BroadcastMessageToFirehoses(msg)

}

func (group *GroupedSinks) BroadCastError(appId string, errorMsg *events.Envelope) {
	group.RLock()
	defer group.RUnlock()

	for _, wrapper := range group.apps[appId] {
		if wrapper.Sink.ShouldReceiveErrors() {
			wrapper.InputChan <- errorMsg
		}
	}

	group.BroadcastMessageToFirehoses(errorMsg)
}

func (group *GroupedSinks) BroadcastMessageToFirehoses(msg *events.Envelope) {
	for _, fgroup := range group.firehoses {
		fgroup.BroadcastMessage(msg)
	}
}

func (group *GroupedSinks) CountFor(appId string) int {
	group.RLock()
	defer group.RUnlock()

	if _, ok := group.apps[appId]; !ok {
		return 0
	}
	return len(group.apps[appId])
}

func (group *GroupedSinks) DrainFor(appId, drainUrl string) sinks.Sink {
	group.RLock()
	defer group.RUnlock()

	wrapper, ok := group.apps[appId][drainUrl]
	if ok {
		return wrapper.Sink
	}
	return nil
}

func (group *GroupedSinks) DrainsFor(appId string) []sinks.Sink {
	group.RLock()
	defer group.RUnlock()

	results := []sinks.Sink{}
	for _, wrapper := range group.apps[appId] {
		_, isSyslogSink := wrapper.Sink.(*syslog.SyslogSink)
		if isSyslogSink {
			results = append(results, wrapper.Sink)
		}
	}

	return results
}

func (group *GroupedSinks) DumpFor(appId string) *dump.DumpSink {
	group.RLock()
	defer group.RUnlock()

	appCache, ok := group.apps[appId]

	if !ok {
		return nil
	}
	if _, ok := appCache[appId]; !ok {

		return nil
	}
	return appCache[appId].Sink.(*dump.DumpSink)
}

func (group *GroupedSinks) WebsocketSinksFor(appId string) []websocket.WebsocketSink {
	results := []websocket.WebsocketSink{}

	group.RLock()
	group.RUnlock()

	for _, wrapper := range group.apps[appId] {
		webSocketSink, isWebsocketSink := wrapper.Sink.(*websocket.WebsocketSink)
		if isWebsocketSink {
			results = append(results, *webSocketSink)
		}
	}

	return results
}

func (group *GroupedSinks) CloseAndDelete(sink sinks.Sink) bool {
	group.Lock()
	defer group.Unlock()
	wrapper, ok := group.apps[sink.AppId()][sink.Identifier()]
	if ok {
		close(wrapper.InputChan)
		delete(group.apps[sink.AppId()], sink.Identifier())
		return true
	}
	return false
}

func (group *GroupedSinks) CloseAndDeleteFirehose(sink sinks.Sink) bool {
	group.Lock()
	defer group.Unlock()
	fgroup, ok := group.firehoses[sink.AppId()]
	if !ok {
		return false
	}

	removed := fgroup.RemoveSink(sink)

	if removed == false {
		return false
	}

	if fgroup.IsEmpty() == true {
		delete(group.firehoses, sink.AppId())
	}

	return true
}

func (group *GroupedSinks) DeleteAll() {
	group.Lock()
	defer group.Unlock()
	for appId, appSinks := range group.apps {
		for _, wrapper := range appSinks {
			close(wrapper.InputChan)
		}
		delete(group.apps, appId)
	}
	for subscriptionId, fgroup := range group.firehoses {
		fgroup.RemoveAllSinks()
		delete(group.firehoses, subscriptionId)
	}
}
