package sinks

import (
	"sync"

	"code.cloudfoundry.org/loggregator/metricemitter"
	"github.com/cloudfoundry/sonde-go/events"
)

type SinkWrapper struct {
	InputChan chan<- *events.Envelope
	Sink      Sink
}

type AppGroup struct {
	mu       sync.RWMutex
	wrappers map[string]*SinkWrapper

	batcher       MetricBatcher
	droppedMetric *metricemitter.Counter
	errorMetric   *metricemitter.Counter
}

func NewAppGroup(
	batcher MetricBatcher,
	droppedMetric *metricemitter.Counter,
	errorMetric *metricemitter.Counter,
) *AppGroup {
	return &AppGroup{
		wrappers:      make(map[string]*SinkWrapper),
		batcher:       batcher,
		droppedMetric: droppedMetric,
		errorMetric:   errorMetric,
	}
}

func (g *AppGroup) AddSink(sink Sink, in chan<- *events.Envelope) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.exists(sink) {
		return false
	}

	g.wrappers[sink.Identifier()] = &SinkWrapper{
		InputChan: in,
		Sink:      sink,
	}

	return true
}

func (g *AppGroup) Exists(sink Sink) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.exists(sink)
}

// exists needs to be called with read or write lock held.
func (g *AppGroup) exists(sink Sink) bool {
	w, ok := g.wrappers[sink.Identifier()]
	return ok && w != nil
}

func (g *AppGroup) Sink(id string) Sink {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.sink(id)
}

func (g *AppGroup) RecentLogsSink(id string) *DumpSink {
	g.mu.RLock()
	defer g.mu.RUnlock()

	dump, ok := g.sink(id).(*DumpSink)
	if !ok {
		return nil
	}
	return dump
}

func (g *AppGroup) ContainerMetricsSink(id string) *ContainerMetricSink {
	g.mu.RLock()
	defer g.mu.RUnlock()

	containerMetrics, ok := g.sink(id).(*ContainerMetricSink)
	if !ok {
		return nil
	}
	return containerMetrics
}

// sink needs to be called with read or write lock held.
func (g *AppGroup) sink(id string) Sink {
	wrapper, ok := g.wrappers[id]
	if !ok || wrapper == nil {
		return nil
	}
	return wrapper.Sink
}

func (g *AppGroup) SyslogSinks() []Sink {
	g.mu.RLock()
	defer g.mu.RUnlock()

	results := []Sink{}
	for _, wrapper := range g.wrappers {
		if wrapper == nil {
			continue
		}
		_, ok := wrapper.Sink.(*SyslogSink)
		if !ok {
			continue
		}
		results = append(results, wrapper.Sink)
	}

	return results
}

func (g *AppGroup) RemoveSink(sink Sink) bool {
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
func (g *AppGroup) removeSink(sink Sink) bool {
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
			// metric-documentation-v1: (sinks.dropped) Number of envelopes dropped
			// while inserting envelope into sink.
			g.batcher.BatchIncrementCounter("sinks.dropped")

			// metric-documentation-v2: (loggregator.doppler.sinks.dropped)
			// Number of envelopes dropped while inserting envelope into sink.
			g.droppedMetric.Increment(1)
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
				// metric-documentation-v1: (sinks.errors.dropped) Number of errors dropped
				// while inserting error into sink.
				g.batcher.BatchIncrementCounter("sinks.errors.dropped")

				// metric-documentation-v2: (loggregator.doppler.sinks.errors.dropped)
				// Number of errors dropped while inserting error into sink.
				g.errorMetric.Increment(1)
			}
		}
	}
}

func (g *AppGroup) length() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.wrappers)
}
