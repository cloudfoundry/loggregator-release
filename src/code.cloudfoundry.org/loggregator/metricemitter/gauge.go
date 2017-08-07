package metricemitter

import (
	"math"
	"sync/atomic"
	"time"

	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"
)

// Gauge stores data about a gauge metric to be used by the metric emitter
// Client.
type Gauge struct {
	Tagged
	name     string
	unit     string
	sourceID string
	value    uint64
}

// NewGauge initializes a new Gauge metric with a given name, unit, sourceID
// and MetricOptions. This should be initialized through the metric emitter
// Client but is exported for making it easier to test metric emission.
func NewGauge(name, unit, sourceID string, opts ...MetricOption) *Gauge {
	m := &Gauge{
		name:     name,
		unit:     unit,
		sourceID: sourceID,
	}
	m.Tagged.tags = make(map[string]*v2.Value)

	for _, opt := range opts {
		opt(m.Tagged)
	}

	return m
}

// Set atomically sets the value of the Gauge.
func (m *Gauge) Set(value float64) {
	atomic.StoreUint64(&m.value, math.Float64bits(value))
}

// GetValue atomically returns the current value of the gauge.
func (m *Gauge) GetValue() float64 {
	return math.Float64frombits(atomic.LoadUint64(&m.value))
}

// WithEnvelope will take in a function that will receive a V2 Envelope. This
// is used by the metric emitter Client when the Gauge metric is send to the
// IngressClient.
func (m *Gauge) WithEnvelope(fn func(*v2.Envelope) error) error {
	v := m.GetValue()

	return fn(m.toEnvelope(v))
}

func (m *Gauge) toEnvelope(value float64) *v2.Envelope {
	return &v2.Envelope{
		SourceId:  m.sourceID,
		Timestamp: time.Now().UnixNano(),
		Message: &v2.Envelope_Gauge{
			Gauge: &v2.Gauge{
				Metrics: map[string]*v2.GaugeValue{
					m.name: {
						Unit:  m.unit,
						Value: m.GetValue(),
					},
				},
			},
		},
		DeprecatedTags: m.tags,
	}
}
