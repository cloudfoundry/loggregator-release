package metricemitter

import (
	"math"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
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
	m.Tagged.tags = make(map[string]*loggregator_v2.Value)

	for _, opt := range opts {
		opt(m.Tagged)
	}

	return m
}

// Set atomically sets the value of the Gauge.
func (m *Gauge) Set(value float64) {
	atomic.StoreUint64(&m.value, toUint64(value, 2))
}

// Increment increments the gauge by a specified value.
func (m *Gauge) Increment(value float64) {
	atomic.AddUint64(&m.value, toUint64(value, 2))
}

// Decrement decrements the gauge by a specified value.
func (m *Gauge) Decrement(value float64) {
	atomic.AddUint64(&m.value, toUint64(-value, 2))
}

// GetValue atomically returns the current value of the gauge.
func (m *Gauge) GetValue() float64 {
	return toFloat64(atomic.LoadUint64(&m.value), 2)
}

// WithEnvelope will take in a function that will receive a V2 Envelope. This
// is used by the metric emitter Client when the Gauge metric is send to the
// IngressClient.
func (m *Gauge) WithEnvelope(fn func(*loggregator_v2.Envelope) error) error {
	v := m.GetValue()

	return fn(m.toEnvelope(v))
}

func (m *Gauge) toEnvelope(value float64) *loggregator_v2.Envelope {
	return &loggregator_v2.Envelope{
		SourceId:  m.sourceID,
		Timestamp: time.Now().UnixNano(),
		Message: &loggregator_v2.Envelope_Gauge{
			Gauge: &loggregator_v2.Gauge{
				Metrics: map[string]*loggregator_v2.GaugeValue{
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

func toFloat64(v uint64, precision int) float64 {
	return float64(v) / math.Pow(10.0, float64(precision))
}

func toUint64(v float64, precision int) uint64 {
	return uint64(v * math.Pow(10.0, float64(precision)))
}
