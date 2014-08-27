package groupedsinks

import (
	"doppler/sinks"
	"doppler/sinks/dump"
	"doppler/sinks/syslog"
	"doppler/sinks/websocket"
	"github.com/cloudfoundry/dropsonde/events"
	"sync"
)

func NewGroupedSinks() *GroupedSinks {
	return &GroupedSinks{apps: make(map[string]map[string]*sinkWrapper)}
}

type sinkWrapper struct {
	inputChan chan<- *events.Envelope
	sink      sinks.Sink
}

type GroupedSinks struct {
	apps map[string]map[string]*sinkWrapper
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
		group.apps[appId] = make(map[string]*sinkWrapper)
		sinksForApp = group.apps[appId]
	}

	if _, ok := sinksForApp[sink.Identifier()]; ok {
		return false
	}
	sinksForApp[sink.Identifier()] = &sinkWrapper{inputChan: in, sink: sink}
	return true
}

func (group *GroupedSinks) BroadCast(appId string, msg *events.Envelope) {
	group.RLock()
	defer group.RUnlock()

	for _, wrapper := range group.apps[appId] {
		wrapper.inputChan <- msg
	}
}

func (gc *GroupedSinks) BroadCastError(appId string, errorMsg *events.Envelope) {
	gc.RLock()
	defer gc.RUnlock()

	for _, wrapper := range gc.apps[appId] {
		if wrapper.sink.ShouldReceiveErrors() {
			wrapper.inputChan <- errorMsg
		}
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
		return wrapper.sink
	}
	return nil
}

func (group *GroupedSinks) DrainsFor(appId string) []sinks.Sink {
	group.RLock()
	defer group.RUnlock()

	results := []sinks.Sink{}
	for _, wrapper := range group.apps[appId] {
		_, isSyslogSink := wrapper.sink.(*syslog.SyslogSink)
		if isSyslogSink {
			results = append(results, wrapper.sink)
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
	return appCache[appId].sink.(*dump.DumpSink)
}

func (group *GroupedSinks) WebsocketSinksFor(appId string) []websocket.WebsocketSink {
	results := []websocket.WebsocketSink{}

	group.RLock()
	group.RUnlock()

	for _, wrapper := range group.apps[appId] {
		webSocketSink, isWebsocketSink := wrapper.sink.(*websocket.WebsocketSink)
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
		close(wrapper.inputChan)
		delete(group.apps[sink.AppId()], sink.Identifier())
		return true
	}
	return false
}

func (group *GroupedSinks) DeleteAll() {
	group.Lock()
	defer group.Unlock()
	for appId, appSinks := range group.apps {
		for _, wrapper := range appSinks {
			close(wrapper.inputChan)
		}
		delete(group.apps, appId)
	}
}
