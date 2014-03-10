package groupedsinks

import (
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"loggregator/sinks"
	"loggregator/sinks/dump"
	"loggregator/sinks/syslog"
	"sync"
)

func NewGroupedSinks() *GroupedSinks {
	return &GroupedSinks{apps: make(map[string]map[string]*sinkWrapper)}
}

type sinkWrapper struct {
	inputChan chan<- *logmessage.Message
	s         sinks.Sink
}

type GroupedSinks struct {
	apps map[string]map[string]*sinkWrapper
	sync.RWMutex
}

func (gc *GroupedSinks) Register(in chan<- *logmessage.Message, s sinks.Sink) bool {
	gc.Lock()
	defer gc.Unlock()

	appId := s.AppId()
	if appId == "" || s.Identifier() == "" {
		return false
	}
	sinksForApp := gc.apps[appId]
	if sinksForApp == nil {
		gc.apps[appId] = make(map[string]*sinkWrapper)
		sinksForApp = gc.apps[appId]
	}

	if _, ok := sinksForApp[s.Identifier()]; ok {
		return false
	}
	sinksForApp[s.Identifier()] = &sinkWrapper{inputChan: in, s: s}
	return true
}

func (gc *GroupedSinks) BroadCast(appId string, msg *logmessage.Message) {
	gc.RLock()
	defer gc.RUnlock()

	for _, wrapper := range gc.apps[appId] {
		//		gc.logger.Debugf("MessageRouter:ParsedMessageChan: Sending Message to channel %v for sinks targeting [%s].", wrapper.s.Identifier(), appId)
		wrapper.inputChan <- msg
	}
}

func (gc *GroupedSinks) BroadCastError(appId string, errorMsg *logmessage.Message) {
	gc.RLock()
	defer gc.RUnlock()

	for _, wrapper := range gc.apps[appId] {
		if wrapper.s.ShouldReceiveErrors() {
			//			sinkManager.logger.Debugf("SinkManager:ErrorChannel: Sending Message to channel %v for sinks targeting [%s].", sink.Identifier(), appId)
			wrapper.inputChan <- errorMsg
		}
	}
}

func (gc *GroupedSinks) CountFor(appId string) int {
	gc.RLock()
	defer gc.RUnlock()

	if _, ok := gc.apps[appId]; !ok {
		return 0
	}
	return len(gc.apps[appId])
}

func (gc *GroupedSinks) DrainFor(appId, drainUrl string) (result sinks.Sink) {
	gc.RLock()
	defer gc.RUnlock()

	wrapper, ok := gc.apps[appId][drainUrl]
	if ok {
		return wrapper.s
	}
	return nil
}

func (gc *GroupedSinks) DrainsFor(appId string) (results []sinks.Sink) {
	gc.RLock()
	defer gc.RUnlock()

	for _, wrapper := range gc.apps[appId] {
		_, isSyslogSink := wrapper.s.(*syslog.SyslogSink)
		if isSyslogSink {
			results = append(results, wrapper.s)
		}
	}

	return results
}

func (gc *GroupedSinks) DumpFor(appId string) *dump.DumpSink {
	gc.RLock()
	defer gc.RUnlock()

	appCache, ok := gc.apps[appId]

	if !ok {
		return nil
	}
	if _, ok := appCache[appId]; !ok {
		return nil
	}
	return appCache[appId].s.(*dump.DumpSink)
}

func (gc *GroupedSinks) Delete(sink sinks.Sink) bool {
	gc.Lock()
	defer gc.Unlock()
	wrapper, ok := gc.apps[sink.AppId()][sink.Identifier()]
	if ok {
		close(wrapper.inputChan)
		delete(gc.apps[sink.AppId()], sink.Identifier())
		return true
	}
	return false
}

func (gc *GroupedSinks) DeleteAll() {
	gc.Lock()
	defer gc.Unlock()
	for appId, appSinks := range gc.apps {
		for _, wrapper := range appSinks {
			close(wrapper.inputChan)
		}
		delete(gc.apps, appId)
	}
}
