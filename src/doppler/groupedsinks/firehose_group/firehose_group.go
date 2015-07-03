package firehose_group

import (
	"doppler/groupedsinks/sink_wrapper"
	"doppler/sinks"
	"sync"

	"github.com/cloudfoundry/sonde-go/events"
)

type FirehoseGroup interface {
	AddSink(sink sinks.Sink, in chan<- *events.Envelope) bool
	Exists(sink sinks.Sink) bool
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

func (group *firehoseGroup) Exists(sink sinks.Sink) bool {
	group.Lock()
	defer group.Unlock()
	for _, sinkWrapper := range group.sinkWrappers {
		if sink.Identifier() == sinkWrapper.Sink.Identifier() {
			return true
		}
	}
	return false
}

func (group *firehoseGroup) AddSink(sink sinks.Sink, in chan<- *events.Envelope) bool {
	if group.Exists(sink) {
		return false
	}

	group.Lock()
	defer group.Unlock()

	sinkWrapper := sink_wrapper.SinkWrapper{InputChan: in, Sink: sink}
	group.sinkWrappers = append(group.sinkWrappers, &sinkWrapper)
	return true
}

func (group *firehoseGroup) RemoveSink(fsink sinks.Sink) bool {
	for i, sinkWrapper := range group.sinkWrappers {
		if sinkWrapper.Sink == fsink {
			group.Lock()
			defer group.Unlock()

			close(sinkWrapper.InputChan)
			s := group.sinkWrappers
			group.sinkWrappers = s[:i+copy(s[i:], s[i+1:])]

			return true
		}
	}

	return false
}

func (group *firehoseGroup) RemoveAllSinks() {
	for _, sinkWrapper := range group.sinkWrappers {
		group.RemoveSink(sinkWrapper.Sink)
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
