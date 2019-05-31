package sinks

import (
	"log"
	"sync"
	"time"

	"github.com/cloudfoundry/sonde-go/events"
)

// SinkManager provides an in memory store of recent logs.
type SinkManager struct {
	metrics        *SinkManagerMetrics
	recentLogCount uint32
	doneChannel    chan struct{}
	errorChannel   chan *events.Envelope
	sinks          *GroupedSinks
	sinkTimeout    time.Duration
	health         HealthRegistrar
	stopOnce       sync.Once
}

// NewSinkManager creates a SinkManager.
func NewSinkManager(
	maxRetainedLogMessages uint32,
	sinkTimeout time.Duration,
	metricClient MetricClient,
	health HealthRegistrar,
) *SinkManager {
	return &SinkManager{
		doneChannel:    make(chan struct{}),
		errorChannel:   make(chan *events.Envelope, 100),
		sinks:          NewGroupedSinks(metricClient),
		recentLogCount: maxRetainedLogMessages,
		metrics:        NewSinkManagerMetrics(metricClient),
		sinkTimeout:    sinkTimeout,
		health:         health,
	}
}

// Stop terminates the sink manager.
func (sm *SinkManager) Stop() {
	sm.stopOnce.Do(func() {
		close(sm.doneChannel)
		sm.sinks.DeleteAll()
	})
}

// SendTo sends an envelope to the registered sinks for a specified
// application ID.
func (sm *SinkManager) SendTo(appID string, msg *events.Envelope) {
	sm.ensureRecentLogsSinkFor(appID)
	sm.sinks.Broadcast(appID, msg)
}

// RegisterSink sink adds a new sink for the sink manager to manage.
//
// FIXME This method should be private. Nothing calls it except for private
// functions in this file.
func (sm *SinkManager) RegisterSink(sink Sink) bool {
	inputChan := make(chan *events.Envelope, 128)
	ok := sm.sinks.RegisterAppSink(inputChan, sink)
	if !ok {
		return false
	}

	// metric-documentation-v1: see sink_manager_metrics.go for details
	sm.metrics.Inc(sink)

	go func() {
		sink.Run(inputChan)
		sm.UnregisterSink(sink)
	}()

	return true
}

// UnregisterSink removes a particular sink from the sink manager.
//
// FIXME This method should be private. Nothing calls it except for private
// functions in this file.
func (sm *SinkManager) UnregisterSink(sink Sink) {
	ok := sm.sinks.CloseAndDelete(sink)
	if !ok {
		return
	}
	sm.metrics.Dec(sink)
}

// RecentLogsFor provides a fixed number of logs for an application ID.
func (sm *SinkManager) RecentLogsFor(appID string) []*events.Envelope {
	if sink := sm.sinks.DumpFor(appID); sink != nil {
		return sink.Dump()
	}

	return nil
}

func (sm *SinkManager) listenForErrorMessages() {
	for {
		select {
		case <-sm.doneChannel:
			return
		case errorMessage, ok := <-sm.errorChannel:
			if !ok {
				return
			}
			appID := getAppId(errorMessage)
			log.Printf("Sink error for %s: %s", appID, errorMessage)
		}
	}
}

func (sm *SinkManager) ensureRecentLogsSinkFor(appID string) {
	if sm.sinks.DumpFor(appID) != nil {
		return
	}

	sink := NewDumpSink(
		appID,
		sm.recentLogCount,
		sm.sinkTimeout,
		sm.health,
	)

	sm.RegisterSink(sink)
}
