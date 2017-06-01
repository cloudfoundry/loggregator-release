package health_test

import (
	"trafficcontroller/internal/health"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
)

var _ = Describe("Health", func() {
	var (
		h           *health.Health
		registrar   *spyRegistrar
		gaugeCount1 *spyGauge
		gaugeCount2 *spyGauge
	)

	BeforeEach(func() {
		gaugeCount1 = newSpyGauge()
		gaugeCount2 = newSpyGauge()
		registrar = newSpyRegistrar()
		h = health.New(registrar, map[string]prometheus.Gauge{
			"count-1": gaugeCount1,
			"count-2": gaugeCount2,
		})
	})

	Describe("Registering gauge values", func() {
		It("registers the gauges with the registrar", func() {
			Expect(registrar.collectors).To(HaveLen(2))
		})
	})

	Describe("Set()", func() {
		It("sets the value on the gauge", func() {
			h.Set("count-1", 30.0)
			Expect(gaugeCount1.value).To(Equal(30.0))

			h.Set("count-2", 60.0)
			Expect(gaugeCount2.value).To(Equal(60.0))
		})
	})
})

type spyRegistrar struct {
	prometheus.Registerer
	collectors []prometheus.Collector
}

func newSpyRegistrar() *spyRegistrar {
	return &spyRegistrar{}
}

func (s *spyRegistrar) MustRegister(c ...prometheus.Collector) {
	s.collectors = append(s.collectors, c...)
}

type spyGauge struct {
	prometheus.Gauge
	value float64
}

func newSpyGauge() *spyGauge {
	return &spyGauge{}
}

func (s *spyGauge) Set(val float64) {
	s.value = val
}
