package containermetric

import (
	"github.com/cloudfoundry/dropsonde/events"
	"sync"
)

type ContainerMetricSink struct {
	applicationId string
	metrics       map[int32]*events.Envelope

	sync.RWMutex
}

func NewContainerMetricSink(applicationId string) *ContainerMetricSink {
	return &ContainerMetricSink{
		applicationId: applicationId,
		metrics:       make(map[int32]*events.Envelope),
	}
}

func (sink *ContainerMetricSink) Run(eventChan <-chan *events.Envelope) {
	for event := range eventChan {
		if event.GetEventType() != events.Envelope_ContainerMetric {
			continue
		}

		sink.updateMetric(event)
	}
}

func (sink *ContainerMetricSink) GetLatest() []*events.Envelope {
	sink.RLock()
	defer sink.RUnlock()

	envelopes := []*events.Envelope{}

	for _, env := range sink.metrics {
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
