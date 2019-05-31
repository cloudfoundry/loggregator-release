package metricemitter

import (
	"fmt"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
)

// Counter stores data about a counter metric to be used by the metric emitter
// Client.
type Counter struct {
	Tagged
	name     string
	sourceID string
	delta    uint64
}

// Tagged is a struct that is embedded into metrics to give them common
// functionality around tags.
type Tagged struct {
	tags map[string]*loggregator_v2.Value
}

// MetricOption is a function that can be passed to a metric on initialization
// that configures the metric.
type MetricOption func(Tagged)

// NewCounter initializes a new Counter metric with a given name, sourceID and
// MetricOptions. This should be initialized through the metric emitter Client
// but is exported for making it easier to test metric emission.
func NewCounter(name, sourceID string, opts ...MetricOption) *Counter {
	m := &Counter{
		name:     name,
		sourceID: sourceID,
	}
	m.Tagged.tags = make(map[string]*loggregator_v2.Value)

	for _, opt := range opts {
		opt(m.Tagged)
	}

	return m
}

// Increment atomically adds the given value to the Counters delta.
func (m *Counter) Increment(c uint64) {
	atomic.AddUint64(&m.delta, c)
}

// GetDelta atomically returns the Counters current delta.
func (m *Counter) GetDelta() uint64 {
	return atomic.LoadUint64(&m.delta)
}

// WithEnvelope will take in a function that will receive a V2 Envelope. This
// is used by the metric emitter Client when the Counter metric is send to the
// IngressClient. When WithEnvelope is called the Counters delta is reset to
// 0, if an error is returned by the given function, the delta will be
// atomically added back to the Counters delta.
func (m *Counter) WithEnvelope(fn func(*loggregator_v2.Envelope) error) error {
	d := atomic.SwapUint64(&m.delta, 0)

	if err := fn(m.toEnvelope(d)); err != nil {
		atomic.AddUint64(&m.delta, d)
		return err
	}

	return nil
}

func (m *Counter) toEnvelope(delta uint64) *loggregator_v2.Envelope {
	return &loggregator_v2.Envelope{
		SourceId:  m.sourceID,
		Timestamp: time.Now().UnixNano(),
		Message: &loggregator_v2.Envelope_Counter{
			Counter: &loggregator_v2.Counter{
				Name:  m.name,
				Delta: delta,
			},
		},
		DeprecatedTags: m.tags,
	}
}

// WithVersion is a MetricOption that can be used to set the metric version.
func WithVersion(major, minor uint) MetricOption {
	return WithTags(map[string]string{
		"metric_version": fmt.Sprintf("%d.%d", major, minor),
	})
}

// WithTags is a MetricOption that is used to set tags on the metrics V2
// Envelope when created.
func WithTags(tags map[string]string) MetricOption {
	return func(m Tagged) {
		for k, v := range tags {
			m.tags[k] = &loggregator_v2.Value{
				Data: &loggregator_v2.Value_Text{
					Text: v,
				},
			}
		}
	}
}
