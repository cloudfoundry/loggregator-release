package healthendpoint

import (
	"log"

	"github.com/prometheus/client_golang/prometheus"
)

// Registrar maintains a list of metrics to be served by the health endpoint
// server.
type Registrar struct {
	gauges map[string]prometheus.Gauge
}

// New returns an initialized health endpoint registrar configured with the
// given prmetheus.Registerer and map of prometheus.Gauges.
func New(registrar prometheus.Registerer, gauges map[string]prometheus.Gauge) *Registrar {
	for _, c := range gauges {
		registrar.MustRegister(c)
	}

	return &Registrar{
		gauges: gauges,
	}
}

// Set will set the given value on the gauge metric with the given name. If
// the gauge metric is not found the process will exit with a status code of
// 1.
func (h *Registrar) Set(name string, value float64) {
	g, ok := h.gauges[name]
	if !ok {
		log.Panicf("Set called for unknown health metric: %s", name)
	}

	g.Set(value)
}

// Inc will increment the gauge metric with the given name by 1. If the gauge
// metric is not found the process will exit with a status code of 1.
func (h *Registrar) Inc(name string) {
	g, ok := h.gauges[name]
	if !ok {
		log.Panicf("Inc called for unknown health metric: %s", name)
	}

	g.Inc()
}

// Dec will decrement the gauge metric with the given name by 1. If the gauge
// metric is not found the process will exit with a status code of 1.
func (h *Registrar) Dec(name string) {
	g, ok := h.gauges[name]
	if !ok {
		log.Panicf("Dec called for unknown health metric: %s", name)
	}

	g.Dec()
}
