package metrics_test

import (
	"expvar"

	"code.cloudfoundry.org/loggregator/rlp-gateway/internal/metrics"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Metrics", func() {
	var (
		spyMap *spyMap
		m      *metrics.Metrics
	)

	BeforeEach(func() {
		spyMap = newSpyMap()
		m = metrics.New(spyMap)
	})

	It("publishes the total of a counter", func() {
		c := m.NewCounter("some-counter")
		c(99)
		c(101)

		Expect(spyMap.getValue("some-counter")).To(Equal(float64(200)))
	})

	It("publishes the value of a gauge", func() {
		c := m.NewGauge("some-gauge")
		c(99.9)
		c(101.1)

		Expect(spyMap.getValue("some-gauge")).To(Equal(101.1))
	})

	It("deals with a nil map", func() {
		m = metrics.New(nil)
		Expect(func() { m.NewGauge("some-gauge") }).ToNot(Panic())
		Expect(func() { m.NewCounter("some-counter") }).ToNot(Panic())
	})
})

type spyMap struct {
	m map[string]expvar.Var
}

func newSpyMap() *spyMap {
	return &spyMap{
		m: make(map[string]expvar.Var),
	}
}

func (s *spyMap) Add(key string, delta int64) {
	s.m[key] = expvar.NewInt(key)
}

func (s *spyMap) AddFloat(key string, delta float64) {
	s.m[key] = expvar.NewFloat(key)
}

func (s *spyMap) Get(key string) expvar.Var {
	return s.m[key]
}

func (s *spyMap) getValue(key string) float64 {
	switch x := s.m[key].(type) {
	case *expvar.Float:
		return x.Value()
	case *expvar.Int:
		return float64(x.Value())
	default:
		panic("Unknown type...")
	}
}
