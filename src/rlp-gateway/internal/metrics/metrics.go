package metrics

import "expvar"

// Metrics stores health metrics for the process. It has a gauge and counter
// metrics.
type Metrics struct {
	m Map
}

// Map stores the desired metrics.
type Map interface {
	// Add adds a new metric to the map.
	Add(key string, delta int64)

	// AddFloat adds a new metric to the map.
	AddFloat(key string, delta float64)

	// Get gets a Var from the Map.
	Get(key string) expvar.Var
}

// New returns a new Metrics.
func New(m Map) *Metrics {
	return &Metrics{
		m: m,
	}
}

// NewCounter returns a func to be used increment the counter total.
func (m *Metrics) NewCounter(name string) func(delta uint64) {
	if m.m == nil {
		return func(_ uint64) {}
	}

	m.m.Add(name, 0)
	i := m.m.Get(name).(*expvar.Int)

	return func(d uint64) {
		i.Add(int64(d))
	}
}

// NewGauge returns a func to be used to set the value of a gauge metric.
func (m *Metrics) NewGauge(name string) func(value float64) {
	if m.m == nil {
		return func(_ float64) {}
	}

	m.m.AddFloat(name, 0)
	f := m.m.Get(name).(*expvar.Float)

	return f.Set
}
