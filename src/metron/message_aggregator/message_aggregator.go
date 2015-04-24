package message_aggregator

import (
	"sync"
	"time"

	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/davecgh/go-spew/spew"
)

var MaxTTL = time.Minute

type MessageAggregator struct {
	startEventsByEventId            map[eventId]startEventEntry
	counterTotals                   map[counterId]uint64
	httpStartReceivedCount          uint64
	httpStopReceivedCount           uint64
	httpStartStopEmittedCount       uint64
	uncategorizedEventCount         uint64
	httpUnmatchedStartReceivedCount uint64
	httpUnmatchedStopReceivedCount  uint64
	counterEventReceivedCount       uint64

	lock   sync.Mutex
	logger *gosteno.Logger
}

func New(logger *gosteno.Logger) *MessageAggregator {
	return &MessageAggregator{
		logger:               logger,
		startEventsByEventId: make(map[eventId]startEventEntry),
		counterTotals:        make(map[counterId]uint64),
	}
}

func (m *MessageAggregator) Run(inputChan <-chan *events.Envelope, outputChan chan<- *events.Envelope) {
	for envelope := range inputChan {
		// TODO: don't call for every message if throughput becomes a problem
		m.cleanupOrphanedHttpStart()

		switch envelope.GetEventType() {
		case events.Envelope_HttpStart:
			m.handleHttpStart(envelope)
		case events.Envelope_HttpStop:
			startStopMessage := m.handleHttpStop(envelope)
			if startStopMessage != nil {
				outputChan <- startStopMessage
			}
		case events.Envelope_CounterEvent:
			counterEventMessage := m.handleCounter(envelope)
			outputChan <- counterEventMessage
		default:
			m.incrementCounter(&m.uncategorizedEventCount)
			m.logger.Debugf("passing through message %v", spew.Sprintf("%v", envelope))
			outputChan <- envelope
		}
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

func (m *MessageAggregator) handleHttpStart(envelope *events.Envelope) {
	m.incrementCounter(&m.httpStartReceivedCount)

	m.logger.Debugf("handling HTTP start message %v", spew.Sprintf("%v", envelope))
	startEvent := envelope.GetHttpStart()

	requestId := startEvent.RequestId.String()
	event := eventId{requestId: requestId, peerType: startEvent.GetPeerType()}
	m.startEventsByEventId[event] = startEventEntry{startEvent: startEvent, entryTime: time.Now()}
}

func (m *MessageAggregator) handleHttpStop(envelope *events.Envelope) *events.Envelope {
	m.incrementCounter(&m.httpStopReceivedCount)

	m.logger.Debugf("handling HTTP stop message %v", spew.Sprintf("%v", envelope))
	stopEvent := envelope.GetHttpStop()

	requestId := stopEvent.RequestId.String()
	event := eventId{requestId: requestId, peerType: stopEvent.GetPeerType()}

	startEventEntry, ok := m.startEventsByEventId[event]
	if !ok {
		m.logger.Warnf("no matching HTTP start message found for %v", event)
		m.incrementCounter(&m.httpUnmatchedStopReceivedCount)
		return nil
	}

	m.incrementCounter(&m.httpStartStopEmittedCount)

	delete(m.startEventsByEventId, event)
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
	m.incrementCounter(&m.counterEventReceivedCount)
	countId := counterId{
		name:   envelope.GetCounterEvent().GetName(),
		origin: envelope.GetOrigin(),
	}

	newVal := m.counterTotals[countId] + envelope.GetCounterEvent().GetDelta()
	m.counterTotals[countId] = newVal

	envelope.GetCounterEvent().Total = &newVal

	return envelope
}

func (m *MessageAggregator) cleanupOrphanedHttpStart() {
	currentTime := time.Now()
	for key, eventEntry := range m.startEventsByEventId {
		if currentTime.Sub(eventEntry.entryTime) > MaxTTL {
			m.incrementCounter(&m.httpUnmatchedStartReceivedCount)
			delete(m.startEventsByEventId, key)
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

type counterId struct {
	origin string
	name   string
}

type eventId struct {
	requestId string
	peerType  events.PeerType
}

type startEventEntry struct {
	startEvent *events.HttpStart
	entryTime  time.Time
}
