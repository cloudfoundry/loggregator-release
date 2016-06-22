package messageaggregator

import (
	"sync"
	"time"

	"metron/writers"

	"github.com/cloudfoundry/dropsonde/logging"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
)

var MaxTTL = time.Minute

type MessageAggregator struct {
	startEventsByEventID map[eventID]startEventEntry
	counterTotals        map[counterID]uint64
	logger               *gosteno.Logger
	outputWriter         writers.EnvelopeWriter
	startEventLock       sync.RWMutex
}

func New(outputWriter writers.EnvelopeWriter, logger *gosteno.Logger) *MessageAggregator {
	return &MessageAggregator{
		logger:               logger,
		outputWriter:         outputWriter,
		startEventsByEventID: make(map[eventID]startEventEntry),
		counterTotals:        make(map[counterID]uint64),
	}
}

func (m *MessageAggregator) Write(envelope *events.Envelope) {
	// TODO: don't call for every message if throughput becomes a problem
	m.cleanupOrphanedHTTPStart()

	if envelope.EventType == nil {
		metrics.BatchIncrementCounter("MessageAggregator.uncategorizedEvents")
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
		metrics.BatchIncrementCounter("MessageAggregator.uncategorizedEvents")
		m.outputWriter.Write(envelope)
	}
}

func (m *MessageAggregator) updateStartEvent(id eventID, entry startEventEntry) {
	m.startEventLock.Lock()
	defer m.startEventLock.Unlock()
	m.startEventsByEventID[id] = entry
}

func (m *MessageAggregator) startEvent(id eventID) (startEventEntry, bool) {
	m.startEventLock.RLock()
	defer m.startEventLock.RUnlock()
	entry, ok := m.startEventsByEventID[id]
	return entry, ok
}

func (m *MessageAggregator) handleHTTPStart(envelope *events.Envelope) {
	metrics.BatchIncrementCounter("MessageAggregator.httpStartReceived")

	logging.Debugf(m.logger, "handling HTTP start message for appID: %v", envelope.GetHttpStart().GetApplicationId())
	startEvent := envelope.GetHttpStart()

	requestID := startEvent.RequestId.String()
	eventID := eventID{requestID: requestID, peerType: startEvent.GetPeerType()}
	m.updateStartEvent(eventID, startEventEntry{startEvent: startEvent, entryTime: time.Now()})
}

func (m *MessageAggregator) handleHTTPStop(envelope *events.Envelope) *events.Envelope {
	metrics.BatchIncrementCounter("MessageAggregator.httpStopReceived")

	logging.Debugf(m.logger, "handling HTTP stop message for appID: %v", envelope.GetHttpStop().GetApplicationId())
	stopEvent := envelope.GetHttpStop()

	requestID := stopEvent.RequestId.String()
	event := eventID{requestID: requestID, peerType: stopEvent.GetPeerType()}

	startEventEntry, ok := m.startEvent(event)
	if !ok {
		m.logger.Warnf("no matching HTTP start message found for appID: %v", envelope.GetHttpStop().GetApplicationId())
		metrics.BatchIncrementCounter("MessageAggregator.httpUnmatchedStopReceived")
		return nil
	}

	metrics.BatchIncrementCounter("MessageAggregator.httpStartStopEmitted")

	delete(m.startEventsByEventID, event)
	startEvent := startEventEntry.startEvent

	return &events.Envelope{
		Origin:    envelope.Origin,
		Timestamp: stopEvent.Timestamp,
		EventType: events.Envelope_HttpStartStop.Enum(),
		HttpStartStop: &events.HttpStartStop{
			StartTimestamp: startEvent.Timestamp,
			StopTimestamp:  stopEvent.Timestamp,
			RequestId:      startEvent.RequestId,
			PeerType:       startEvent.PeerType,
			Method:         startEvent.Method,
			Uri:            startEvent.Uri,
			RemoteAddress:  startEvent.RemoteAddress,
			UserAgent:      startEvent.UserAgent,
			StatusCode:     stopEvent.StatusCode,
			ContentLength:  stopEvent.ContentLength,
			ApplicationId:  stopEvent.ApplicationId,
			InstanceIndex:  startEvent.InstanceIndex,
			InstanceId:     startEvent.InstanceId,
		},
	}
}

func (m *MessageAggregator) handleCounter(envelope *events.Envelope) *events.Envelope {
	metrics.BatchIncrementCounter("MessageAggregator.counterEventReceived")

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
	m.startEventLock.Lock()
	defer m.startEventLock.Unlock()
	currentTime := time.Now()
	for key, eventEntry := range m.startEventsByEventID {
		if currentTime.Sub(eventEntry.entryTime) > MaxTTL {
			metrics.BatchIncrementCounter("MessageAggregator.httpUnmatchedStartReceived")
			delete(m.startEventsByEventID, key)
		}
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
