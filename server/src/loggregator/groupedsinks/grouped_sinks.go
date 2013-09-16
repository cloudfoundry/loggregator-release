package groupedsinks

import (
	"loggregator/sinks"
	"sync"
)

func NewGroupedSinks() *GroupedSinks {
	return &GroupedSinks{make(map[string]*node), new(sync.RWMutex)}
}

func newNode() *node {
	return &node{sinkSet: make(map[sinks.Sink]bool)}
}

type node struct {
	sinkSet map[sinks.Sink]bool
}

func (n *node) addSink(s sinks.Sink) {
	n.sinkSet[s] = true
}

type GroupedSinks struct {
	apps map[string]*node
	*sync.RWMutex
}

func (gc *GroupedSinks) Register(s sinks.Sink, appId string) {
	gc.Lock()
	defer gc.Unlock()

	if appId != "" {
		app, found := gc.apps[appId]
		if !found {
			app = newNode()
			gc.apps[appId] = app
		}
		app.addSink(s)
	}
}

func (gc *GroupedSinks) For(appId string) (results []sinks.Sink) {
	gc.RLock()
	defer gc.RUnlock()

	results = make([]sinks.Sink, 0)

	if app, found := gc.apps[appId]; found {
		for c, _ := range app.sinkSet {
			results = append(results, c)
		}
	}

	return results
}

func (gc *GroupedSinks) DrainsFor(appId string) (results []sinks.Sink) {
	gc.RLock()
	defer gc.RUnlock()

	results = make([]sinks.Sink, 0)

	if app, found := gc.apps[appId]; found {
		for s, _ := range app.sinkSet {
			_, isSyslogSink := s.(*sinks.SyslogSink)
			if isSyslogSink {
				results = append(results, s)
			}
		}
	}

	return results
}

func (gc *GroupedSinks) DumpFor(appId string) *sinks.DumpSink {
	gc.RLock()
	defer gc.RUnlock()

	if app, found := gc.apps[appId]; found {
		for s, _ := range app.sinkSet {
			_, isDumpSink := s.(*sinks.DumpSink)
			if isDumpSink {
				return s.(*sinks.DumpSink)
			}
		}
	}

	return nil
}

func (gc *GroupedSinks) DrainFor(appId, drainUrl string) (result sinks.Sink) {
	gc.RLock()
	defer gc.RUnlock()

	if app, found := gc.apps[appId]; found {
		for s, _ := range app.sinkSet {
			if s.Identifier() == drainUrl {
				return s
			}
		}
	}
	return nil
}

func (gc *GroupedSinks) Delete(s sinks.Sink) {
	gc.Lock()
	defer gc.Unlock()
	for _, app := range gc.apps {
		delete(app.sinkSet, s)
	}
}
