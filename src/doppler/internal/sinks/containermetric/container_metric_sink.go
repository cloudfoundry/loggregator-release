package containermetric

import (
	"sync"
	"time"

	"github.com/cloudfoundry/sonde-go/events"
)

type ContainerMetricSink struct {
	appID              string
	ttl                time.Duration
	metrics            map[int32]*events.Envelope
	inactivityDuration time.Duration
	lock               sync.RWMutex
}

func NewContainerMetricSink(appID string, ttl time.Duration, inactivityDuration time.Duration) *ContainerMetricSink {
	return &ContainerMetricSink{
		appID:              appID,
		ttl:                ttl,
		inactivityDuration: inactivityDuration,
		metrics:            make(map[int32]*events.Envelope),
	}
}

func (sink *ContainerMetricSink) Run(eventChan <-chan *events.Envelope) {

	timer := time.NewTimer(sink.inactivityDuration)
	for {
		timer.Reset(sink.inactivityDuration)
		select {

		case event, ok := <-eventChan:
			if !ok {
				return
			}

			if event.GetEventType() != events.Envelope_ContainerMetric {
				continue
			}

			sink.updateMetric(event)
		case <-timer.C:
			timer.Stop()
			return
		}
	}
}

func (sink *ContainerMetricSink) GetLatest() []*events.Envelope {
	sink.lock.Lock()
	defer sink.lock.Unlock()

	envelopes := []*events.Envelope{}

	earliestLiveTimestamp := time.Now().Add(-sink.ttl)

	for instanceIndex, env := range sink.metrics {
		metricTimestamp := time.Unix(0, env.GetTimestamp())

		if metricTimestamp.Before(earliestLiveTimestamp) {
			delete(sink.metrics, instanceIndex)
			continue
		}

		envelopes = append(envelopes, env)
	}

	return envelopes
}

func (sink *ContainerMetricSink) AppID() string {
	return sink.appID
}

func (sink *ContainerMetricSink) Identifier() string {
	return "container-metrics-" + sink.appID
}

func (sink *ContainerMetricSink) ShouldReceiveErrors() bool {
	return false
}

func (sink *ContainerMetricSink) updateMetric(event *events.Envelope) {
	sink.lock.Lock()
	defer sink.lock.Unlock()

	instance := event.GetContainerMetric().GetInstanceIndex()

	oldMetric, ok := sink.metrics[instance]

	if !ok || oldMetric.GetTimestamp() < event.GetTimestamp() {
		sink.metrics[instance] = event
	}
}
