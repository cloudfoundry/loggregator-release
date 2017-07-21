package metricemitter

import (
	"fmt"
	"sync/atomic"
	"time"

	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"
)

type Counter struct {
	Tagged
	name     string
	sourceID string
	delta    uint64
}

type Tagged struct {
	tags map[string]*v2.Value
}

type MetricOption func(Tagged)

func NewCounter(name, sourceID string, opts ...MetricOption) *Counter {
	m := &Counter{
		name:     name,
		sourceID: sourceID,
	}
	m.Tagged.tags = make(map[string]*v2.Value)

	for _, opt := range opts {
		opt(m.Tagged)
	}

	return m
}

func (m *Counter) Increment(c uint64) {
	atomic.AddUint64(&m.delta, c)
}

func (m *Counter) GetDelta() uint64 {
	return atomic.LoadUint64(&m.delta)
}

func (m *Counter) WithEnvelope(fn func(*v2.Envelope) error) error {
	d := atomic.SwapUint64(&m.delta, 0)

	if err := fn(m.toEnvelope(d)); err != nil {
		atomic.AddUint64(&m.delta, d)
		return err
	}

	return nil
}

func (m *Counter) toEnvelope(delta uint64) *v2.Envelope {
	return &v2.Envelope{
		SourceId:  m.sourceID,
		Timestamp: time.Now().UnixNano(),
		Message: &v2.Envelope_Counter{
			Counter: &v2.Counter{
				Name: m.name,
				Value: &v2.Counter_Delta{
					Delta: delta,
				},
			},
		},
		DeprecatedTags: m.tags,
	}
}

func WithVersion(major, minor uint) MetricOption {
	return WithTags(map[string]string{
		"metric_version": fmt.Sprintf("%d.%d", major, minor),
	})
}

func WithTags(tags map[string]string) MetricOption {
	return func(m Tagged) {
		for k, v := range tags {
			m.tags[k] = &v2.Value{
				Data: &v2.Value_Text{
					Text: v,
				},
			}
		}
	}
}
