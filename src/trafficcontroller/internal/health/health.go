package health

import (
	"log"

	"github.com/prometheus/client_golang/prometheus"
)

type Health struct {
	gauges map[string]prometheus.Gauge
}

func New(registrar prometheus.Registerer, gauges map[string]prometheus.Gauge) *Health {
	for _, c := range gauges {
		registrar.MustRegister(c)
	}

	return &Health{
		gauges: gauges,
	}
}

func (h *Health) Set(name string, value float64) {
	c, ok := h.gauges[name]
	if !ok {
		log.Panicf("set called for unknown health metric: %s", name)
	}

	c.Set(value)
}
