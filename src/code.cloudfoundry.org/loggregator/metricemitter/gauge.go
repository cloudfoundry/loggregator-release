package metricemitter

import (
	"math"
	"sync/atomic"
	"time"

	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"
)

type Gauge struct {
	Tagged
	name     string
	unit     string
	sourceID string
	value    uint64
}

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

func (m *Gauge) Set(value float64) {
	atomic.StoreUint64(&m.value, math.Float64bits(value))
}

func (m *Gauge) GetValue() float64 {
	return math.Float64frombits(atomic.LoadUint64(&m.value))
}

func (m *Gauge) WithEnvelope(fn func(*v2.Envelope) error) error {
	v := m.GetValue()
	if err := fn(m.toEnvelope(v)); err != nil {
		return err
	}

	return nil
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
