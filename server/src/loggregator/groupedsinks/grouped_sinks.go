package groupedsinks

import (
	"loggregator/sinks"
	"sync"
)

func NewGroupedSinks() *GroupedSinks {
	return &GroupedSinks{make(map[string]map[string]sinks.Sink), new(sync.RWMutex)}
}

type GroupedSinks struct {
	apps map[string]map[string]sinks.Sink
	*sync.RWMutex
}

func (gc *GroupedSinks) Register(s sinks.Sink) bool {
	gc.Lock()
	defer gc.Unlock()

	appId := s.AppId()
	if appId == "" || s.Identifier() == "" {
		return false
	}
	if gc.apps[appId] == nil {
		gc.apps[appId] = make(map[string]sinks.Sink)
	}

	if gc.apps[appId][s.Identifier()] != nil {
		return false
	}
	gc.apps[appId][s.Identifier()] = s
	return true
}

func (gc *GroupedSinks) For(appId string) (results []sinks.Sink) {
	gc.RLock()
	defer gc.RUnlock()

	for _, sink := range gc.apps[appId] {
		results = append(results, sink)
	}

	return results
}

func (gc *GroupedSinks) DrainsFor(appId string) (results []sinks.Sink) {
	gc.RLock()
	defer gc.RUnlock()

	for _, s := range gc.apps[appId] {
		_, isSyslogSink := s.(*sinks.SyslogSink)
		if isSyslogSink {
			results = append(results, s)
		}
	}

	return results
}

func (gc *GroupedSinks) DumpFor(appId string) *sinks.DumpSink {
	gc.RLock()
	defer gc.RUnlock()

	if gc.apps[appId][appId] == nil {
		return nil
	}
	return gc.apps[appId][appId].(*sinks.DumpSink)
}

func (gc *GroupedSinks) DrainFor(appId, drainUrl string) (result sinks.Sink) {
	gc.RLock()
	defer gc.RUnlock()

	return gc.apps[appId][drainUrl]
}

func (gc *GroupedSinks) Delete(sink sinks.Sink) {
	gc.Lock()
	defer gc.Unlock()

	delete(gc.apps[sink.AppId()], sink.Identifier())
}
