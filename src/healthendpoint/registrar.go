package healthendpoint

import (
	"log"

	"github.com/prometheus/client_golang/prometheus"
)

type Registrar struct {
	gauges map[string]prometheus.Gauge
}

func New(registrar prometheus.Registerer, gauges map[string]prometheus.Gauge) *Registrar {
	for _, c := range gauges {
		registrar.MustRegister(c)
	}

	return &Registrar{
		gauges: gauges,
	}
}

func (h *Registrar) Set(name string, value float64) {
	c, ok := h.gauges[name]
	if !ok {
		log.Panicf("set called for unknown health metric: %s", name)
	}

	c.Set(value)
}
