package healthendpoint_test

import (
	"code.cloudfoundry.org/loggregator/healthendpoint"

	"github.com/prometheus/client_golang/prometheus"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Health", func() {
	var (
		h           *healthendpoint.Registrar
		registrar   *spyRegistrar
		gaugeCount1 *spyGauge
		gaugeCount2 *spyGauge
	)

	BeforeEach(func() {
		gaugeCount1 = newSpyGauge()
		gaugeCount2 = newSpyGauge()
		registrar = newSpyRegistrar()
		h = healthendpoint.New(registrar, map[string]prometheus.Gauge{
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

	Describe("Inc()", func() {
		It("Increments the gauge", func() {
			h.Inc("count-1")
			Expect(gaugeCount1.inc).To(Equal(1))

			h.Inc("count-2")
			Expect(gaugeCount2.inc).To(Equal(1))
		})
	})

	Describe("Dec()", func() {
		It("Decrements the gauge", func() {
			h.Dec("count-1")
			Expect(gaugeCount1.dec).To(Equal(1))

			h.Dec("count-2")
			Expect(gaugeCount2.dec).To(Equal(1))
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
	inc   int
	dec   int
}

func newSpyGauge() *spyGauge {
	return &spyGauge{}
}

func (s *spyGauge) Set(val float64) {
	s.value = val
}

func (s *spyGauge) Inc() {
	s.inc++
}

func (s *spyGauge) Dec() {
	s.dec++
}
