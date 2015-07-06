package messageaggregator

import (
	"sync"
	"time"

	"metron/writers"

	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/davecgh/go-spew/spew"
)

var MaxTTL = time.Minute

type MessageAggregator struct {
	startEventsByEventID            map[eventID]startEventEntry
	counterTotals                   map[counterID]uint64
	httpStartReceivedCount          uint64
	httpStopReceivedCount           uint64
	httpStartStopEmittedCount       uint64
	uncategorizedEventCount         uint64
	httpUnmatchedStartReceivedCount uint64
	httpUnmatchedStopReceivedCount  uint64
	counterEventReceivedCount       uint64
	emitMetrics                     bool

	lock sync.Mutex

	logger       *gosteno.Logger
	outputWriter writers.EnvelopeWriter
}

func New(outputWriter writers.EnvelopeWriter, logger *gosteno.Logger, emit bool) *MessageAggregator {
	return &MessageAggregator{
		logger:               logger,
		outputWriter:         outputWriter,
		startEventsByEventID: make(map[eventID]startEventEntry),
		counterTotals:        make(map[counterID]uint64),
		emitMetrics:          emit,
	}
}

func (m *MessageAggregator) Write(envelope *events.Envelope) {
	// TODO: don't call for every message if throughput becomes a problem
	m.cleanupOrphanedHTTPStart()

	if envelope.EventType == nil {
		m.outputWriter.Write(envelope)
		return
	}
	switch envelope.GetEventType() {
	case events.Envelope_HttpStart:
		m.handleHTTPStart(envelope)
	case events.Envelope_HttpStop:
		startStopMessage := m.handleHTTPStop(envelope)
		if startStopMessage != nil {
			m.outputWriter.Write(startStopMessage)
		}
	case events.Envelope_CounterEvent:
		counterEventMessage := m.handleCounter(envelope)
		m.outputWriter.Write(counterEventMessage)
	default:
		m.incrementCounter(&m.uncategorizedEventCount)
		if m.emitMetrics {
			metrics.IncrementCounter("MessageAggregator.uncategorizedEvents")
		}
		m.logger.Debugf("passing through message %v", spew.Sprintf("%v", envelope))
		m.outputWriter.Write(envelope)
	}
}

func (m *MessageAggregator) Emit() instrumentation.Context {
	return instrumentation.Context{
		Name:    "MessageAggregator",
		Metrics: m.metrics(),
	}
}

func (m *MessageAggregator) incrementCounter(counter *uint64) {
	m.lock.Lock()
	defer m.lock.Unlock()
	(*counter)++
}

func (m *MessageAggregator) handleHTTPStart(envelope *events.Envelope) {
	if m.emitMetrics {
		metrics.IncrementCounter("MessageAggregator.httpStartReceived")
	}
	m.incrementCounter(&m.httpStartReceivedCount)

	m.logger.Debugf("handling HTTP start message %v", spew.Sprintf("%v", envelope))
	startEvent := envelope.GetHttpStart()

	requestID := startEvent.RequestId.String()
	event := eventID{requestID: requestID, peerType: startEvent.GetPeerType()}
	m.startEventsByEventID[event] = startEventEntry{startEvent: startEvent, entryTime: time.Now()}
}

func (m *MessageAggregator) handleHTTPStop(envelope *events.Envelope) *events.Envelope {
	if m.emitMetrics {
		metrics.IncrementCounter("MessageAggregator.httpStopReceived")
	}
	m.incrementCounter(&m.httpStopReceivedCount)

	m.logger.Debugf("handling HTTP stop message %v", spew.Sprintf("%v", envelope))
	stopEvent := envelope.GetHttpStop()

	requestID := stopEvent.RequestId.String()
	event := eventID{requestID: requestID, peerType: stopEvent.GetPeerType()}

	startEventEntry, ok := m.startEventsByEventID[event]
	if !ok {
		m.logger.Warnf("no matching HTTP start message found for %v", event)
		if m.emitMetrics {
			metrics.IncrementCounter("MessageAggregator.httpUnmatchedStopReceived")
		}
		m.incrementCounter(&m.httpUnmatchedStopReceivedCount)
		return nil
	}

	if m.emitMetrics {
		metrics.IncrementCounter("MessageAggregator.httpStartStopEmitted")
	}
	m.incrementCounter(&m.httpStartStopEmittedCount)

	delete(m.startEventsByEventID, event)
	startEvent := startEventEntry.startEvent

	return &events.Envelope{
		Origin:    envelope.Origin,
		Timestamp: stopEvent.Timestamp,
		EventType: events.Envelope_HttpStartStop.Enum(),
		HttpStartStop: &events.HttpStartStop{
			StartTimestamp:  startEvent.Timestamp,
			StopTimestamp:   stopEvent.Timestamp,
			RequestId:       startEvent.RequestId,
			PeerType:        startEvent.PeerType,
			Method:          startEvent.Method,
			Uri:             startEvent.Uri,
			RemoteAddress:   startEvent.RemoteAddress,
			UserAgent:       startEvent.UserAgent,
			StatusCode:      stopEvent.StatusCode,
			ContentLength:   stopEvent.ContentLength,
			ParentRequestId: startEvent.ParentRequestId,
			ApplicationId:   stopEvent.ApplicationId,
			InstanceIndex:   startEvent.InstanceIndex,
			InstanceId:      startEvent.InstanceId,
		},
	}
}

func (m *MessageAggregator) handleCounter(envelope *events.Envelope) *events.Envelope {
	if m.emitMetrics {
		metrics.IncrementCounter("MessageAggregator.counterEventReceived")
	}

	m.incrementCounter(&m.counterEventReceivedCount)
	countID := counterID{
		name:   envelope.GetCounterEvent().GetName(),
		origin: envelope.GetOrigin(),
	}

	newVal := m.counterTotals[countID] + envelope.GetCounterEvent().GetDelta()
	m.counterTotals[countID] = newVal

	envelope.GetCounterEvent().Total = &newVal
	return envelope
}

func (m *MessageAggregator) cleanupOrphanedHTTPStart() {
	currentTime := time.Now()
	for key, eventEntry := range m.startEventsByEventID {
		if currentTime.Sub(eventEntry.entryTime) > MaxTTL {
			if m.emitMetrics {
				metrics.IncrementCounter("MessageAggregator.httpUnmatchedStartReceived")
			}
			m.incrementCounter(&m.httpUnmatchedStartReceivedCount)
			delete(m.startEventsByEventID, key)
		}
	}
}

func (m *MessageAggregator) metrics() []instrumentation.Metric {
	m.lock.Lock()
	defer m.lock.Unlock()

	return []instrumentation.Metric{
		instrumentation.Metric{Name: "httpStartReceived", Value: m.httpStartReceivedCount},
		instrumentation.Metric{Name: "httpStopReceived", Value: m.httpStopReceivedCount},
		instrumentation.Metric{Name: "httpStartStopEmitted", Value: m.httpStartStopEmittedCount},
		instrumentation.Metric{Name: "uncategorizedEvents", Value: m.uncategorizedEventCount},
		instrumentation.Metric{Name: "httpUnmatchedStartReceived", Value: m.httpUnmatchedStartReceivedCount},
		instrumentation.Metric{Name: "httpUnmatchedStopReceived", Value: m.httpUnmatchedStopReceivedCount},
		instrumentation.Metric{Name: "counterEventReceived", Value: m.counterEventReceivedCount},
	}
}

type counterID struct {
	origin string
	name   string
}

type eventID struct {
	requestID string
	peerType  events.PeerType
}

type startEventEntry struct {
	startEvent *events.HttpStart
	entryTime  time.Time
}
