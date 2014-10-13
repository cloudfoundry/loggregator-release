package firehose_group

import (
	"doppler/groupedsinks/sink_wrapper"
	"doppler/sinks"
	"github.com/cloudfoundry/dropsonde/events"
	"sync"
)

type FirehoseGroup interface {
	AddSink(swrapper *sink_wrapper.SinkWrapper) bool
	RemoveSink(fsink sinks.Sink) bool
	RemoveAllSinks()
	IsEmpty() bool
	BroadcastMessage(msg *events.Envelope)
}

type firehoseGroup struct {
	sinkWrappers      []*sink_wrapper.SinkWrapper
	lastUsedSinkIndex int
	sync.RWMutex
}

func NewFirehoseGroup() *firehoseGroup {
	return &firehoseGroup{
		sinkWrappers: make([]*sink_wrapper.SinkWrapper, 0),
	}
}

func (group *firehoseGroup) AddSink(swrapper *sink_wrapper.SinkWrapper) bool {
	for _, fsink := range group.sinkWrappers {
		if swrapper.Sink.Identifier() == fsink.Sink.Identifier() {
			return false
		}
	}

	group.Lock()
	defer group.Unlock()

	group.sinkWrappers = append(group.sinkWrappers, swrapper)
	return true
}

func (group *firehoseGroup) RemoveSink(fsink sinks.Sink) bool {
	for i, swrapper := range group.sinkWrappers {
		if swrapper.Sink == fsink {
			group.Lock()
			defer group.Unlock()

			close(swrapper.InputChan)
			s := group.sinkWrappers
			group.sinkWrappers = s[:i+copy(s[i:], s[i+1:])]

			return true
		}
	}

	return false
}

func (group *firehoseGroup) RemoveAllSinks() {
	for _, swrapper := range group.sinkWrappers {
		group.RemoveSink(swrapper.Sink)
	}
}

func (group *firehoseGroup) IsEmpty() bool {
	return group.length() == 0
}

func (group *firehoseGroup) BroadcastMessage(msg *events.Envelope) {
	group.Lock()
	defer group.Unlock()

	l := len(group.sinkWrappers)

	lastUsed := group.lastUsedSinkIndex

	if lastUsed >= l {
		group.lastUsedSinkIndex = 0
	}

	group.sinkWrappers[group.lastUsedSinkIndex].InputChan <- msg

	group.lastUsedSinkIndex += 1
}

func (group *firehoseGroup) length() int {
	group.RLock()
	defer group.RUnlock()

	return len(group.sinkWrappers)
}
