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
	g, ok := h.gauges[name]
	if !ok {
		log.Panicf("Set called for unknown health metric: %s", name)
	}

	g.Set(value)
}

func (h *Registrar) Inc(name string) {
	g, ok := h.gauges[name]
	if !ok {
		log.Panicf("Inc called for unknown health metric: %s", name)
	}

	g.Inc()
}

func (h *Registrar) Dec(name string) {
	g, ok := h.gauges[name]
	if !ok {
		log.Panicf("Dec called for unknown health metric: %s", name)
	}

	g.Dec()
}
