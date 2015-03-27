package firehose_group

import (
	"doppler/groupedsinks/sink_wrapper"
	"doppler/sinks"
	"sync"

	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/gosteno"
)

type FirehoseGroup interface {
	AddSink(sink sinks.Sink, in chan<- *events.Envelope) bool
	RemoveSink(fsink sinks.Sink) bool
	RemoveAllSinks()
	IsEmpty() bool
	BroadcastMessage(msg *events.Envelope)
}

type firehoseGroup struct {
	logger            *gosteno.Logger
	sinkWrappers      []*sink_wrapper.SinkWrapper
	lastUsedSinkIndex int
	sync.RWMutex
}

func NewFirehoseGroup(logger *gosteno.Logger) *firehoseGroup {
	return &firehoseGroup{
		logger:       logger,
		sinkWrappers: make([]*sink_wrapper.SinkWrapper, 0),
	}
}

func (group *firehoseGroup) AddSink(sink sinks.Sink, in chan<- *events.Envelope) bool {
	for _, sinkWrapper := range group.sinkWrappers {
		if sink.Identifier() == sinkWrapper.Sink.Identifier() {
			return false
		}
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

	select {
	case group.sinkWrappers[group.lastUsedSinkIndex].InputChan <- msg:
	default:
		// don't add the message because there is no consumer
		sinkIdentifier := group.sinkWrappers[group.lastUsedSinkIndex].Sink.Identifier()
		group.logger.Debugf("No firehose consumer, dropping message for sink: %s", sinkIdentifier)
	}

	group.lastUsedSinkIndex += 1
}

func (group *firehoseGroup) length() int {
	group.RLock()
	defer group.RUnlock()

	return len(group.sinkWrappers)
}
