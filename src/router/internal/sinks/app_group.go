package sinks

import (
	"sync"

	"code.cloudfoundry.org/loggregator/metricemitter"
	"github.com/cloudfoundry/sonde-go/events"
)

type Sink interface {
	AppID() string
	Run(<-chan *events.Envelope)
	Identifier() string
}

type SinkWrapper struct {
	InputChan chan<- *events.Envelope
	Sink      Sink
}

type AppGroup struct {
	mu       sync.RWMutex
	wrappers map[string]*SinkWrapper

	droppedMetric *metricemitter.Counter
	errorMetric   *metricemitter.Counter
}

func NewAppGroup(
	droppedMetric *metricemitter.Counter,
	errorMetric *metricemitter.Counter,
) *AppGroup {
	return &AppGroup{
		wrappers:      make(map[string]*SinkWrapper),
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

// sink needs to be called with read or write lock held.
func (g *AppGroup) sink(id string) Sink {
	wrapper, ok := g.wrappers[id]
	if !ok || wrapper == nil {
		return nil
	}
	return wrapper.Sink
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
	}
}

func (g *AppGroup) length() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.wrappers)
}
