package containermetric

import (
	"sync"
	"time"

	"doppler/sinks"
	"github.com/cloudfoundry/dropsonde/events"
)

type ContainerMetricSink struct {
	applicationId       string
	ttl                 time.Duration
	metrics             map[int32]*events.Envelope
	inactivityDuration  time.Duration

	sinks.DropCounter

	sync.RWMutex
}

func NewContainerMetricSink(applicationId string, ttl time.Duration, inactivityDuration time.Duration) *ContainerMetricSink {
	return &ContainerMetricSink{
		applicationId:      applicationId,
		ttl:                ttl,
		inactivityDuration: inactivityDuration,
		metrics:            make(map[int32]*events.Envelope),
		DropCounter: sinks.NewDropCounter(applicationId, "ContainerMetricSink"),
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
	sink.Lock()
	defer sink.Unlock()

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

func (sink *ContainerMetricSink) StreamId() string {
	return sink.applicationId
}

func (sink *ContainerMetricSink) Identifier() string {
	return "container-metrics-" + sink.applicationId
}

func (sink *ContainerMetricSink) ShouldReceiveErrors() bool {
	return false
}

func (sink *ContainerMetricSink) updateMetric(event *events.Envelope) {
	sink.Lock()
	defer sink.Unlock()

	instance := event.GetContainerMetric().GetInstanceIndex()

	oldMetric, ok := sink.metrics[instance]

	if !ok || oldMetric.GetTimestamp() < event.GetTimestamp() {
		sink.metrics[instance] = event
	}
}
