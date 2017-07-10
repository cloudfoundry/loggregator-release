package firehose_group

import (
	"math/rand"
	"sync"

	"code.cloudfoundry.org/loggregator/doppler/internal/groupedsinks/sink_wrapper"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinks"
	"code.cloudfoundry.org/loggregator/metricemitter"

	"github.com/cloudfoundry/sonde-go/events"
)

type MetricBatcher interface {
	BatchIncrementCounter(name string)
}

type FirehoseGroup interface {
	AddSink(sink sinks.Sink, in chan<- *events.Envelope) bool
	Exists(sink sinks.Sink) bool
	RemoveSink(fsink sinks.Sink) bool
	RemoveAllSinks()
	IsEmpty() bool
	BroadcastMessage(msg *events.Envelope)
}

type firehoseGroup struct {
	mu       sync.RWMutex
	wrappers map[string]*sink_wrapper.SinkWrapper

	batcher       MetricBatcher
	droppedMetric *metricemitter.Counter
}

func NewFirehoseGroup(
	batcher MetricBatcher,
	droppedMetric *metricemitter.Counter,
) *firehoseGroup {
	return &firehoseGroup{
		wrappers:      make(map[string]*sink_wrapper.SinkWrapper),
		batcher:       batcher,
		droppedMetric: droppedMetric,
	}
}

func (group *firehoseGroup) Exists(sink sinks.Sink) bool {
	group.mu.RLock()
	defer group.mu.RUnlock()
	return group.exists(sink)
}

func (group *firehoseGroup) AddSink(sink sinks.Sink, in chan<- *events.Envelope) bool {
	group.mu.Lock()
	defer group.mu.Unlock()

	if group.exists(sink) {
		return false
	}

	group.wrappers[sink.Identifier()] = &sink_wrapper.SinkWrapper{
		InputChan: in,
		Sink:      sink,
	}

	return true
}

// exists needs to be called with read or write lock held.
func (group *firehoseGroup) exists(sink sinks.Sink) bool {
	w, ok := group.wrappers[sink.Identifier()]
	return ok && w != nil
}

func (group *firehoseGroup) RemoveSink(fsink sinks.Sink) bool {
	group.mu.Lock()
	defer group.mu.Unlock()
	return group.removeSink(fsink)
}

func (group *firehoseGroup) RemoveAllSinks() {
	group.mu.Lock()
	defer group.mu.Unlock()

	for _, wrapper := range group.wrappers {
		if wrapper != nil {
			group.removeSink(wrapper.Sink)
		}
	}
}

// removeSink needs to be called with write lock held.
func (group *firehoseGroup) removeSink(sink sinks.Sink) bool {
	wrapper, ok := group.wrappers[sink.Identifier()]
	delete(group.wrappers, sink.Identifier())

	if !ok || wrapper == nil {
		return false
	}

	close(wrapper.InputChan)
	return true
}

func (group *firehoseGroup) IsEmpty() bool {
	group.mu.RLock()
	defer group.mu.RUnlock()
	return len(group.wrappers) == 0
}

func (group *firehoseGroup) BroadcastMessage(msg *events.Envelope) {
	group.mu.RLock()
	defer group.mu.RUnlock()

	// only write to a single wrapper
	wrapper := group.randomWrapper()
	if wrapper == nil {
		return
	}
	select {
	case wrapper.InputChan <- msg:
	default:
		// metric-documentation-v1: (sinks.dropped) Number of envelopes dropped
		// while inserting envelope into sink.
		group.batcher.BatchIncrementCounter("sinks.dropped")

		// metric-documentation-v2: (loggregator.doppler.sinks.dropped)
		// Number of envelopes dropped while inserting envelope into sink.
		group.droppedMetric.Increment(1)
	}
}

// randomWrapper needs to be called with read or write lock held.
func (group *firehoseGroup) randomWrapper() *sink_wrapper.SinkWrapper {
	var i int
	if len(group.wrappers) == 0 {
		return nil
	}
	r := rand.Intn(len(group.wrappers))
	for _, w := range group.wrappers {
		if i == r {
			return w
		}
		i++
	}
	return nil
}
