package web_test

import (
	"net/url"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator/rlp-gateway/internal/web"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("SelectorBuilder", func() {
	DescribeTable("single type selectors",
		func(t string, selector *loggregator_v2.Selector) {
			s, err := web.BuildSelector(url.Values{t: {}})
			Expect(err).ToNot(HaveOccurred())

			Expect(s).To(Equal([]*loggregator_v2.Selector{
				selector,
			}))
		},
		Entry("log", "log", buildSelector("log", "")),
		Entry("counter", "counter", buildSelector("counter", "")),
		Entry("gauge", "gauge", buildSelector("gauge", "")),
		Entry("timer", "timer", buildSelector("timer", "")),
		Entry("event", "event", buildSelector("event", "")),
	)

	DescribeTable("single type selectors with multiple source IDs",
		func(t string, sourceIDs []string, selectors []*loggregator_v2.Selector) {
			s, err := web.BuildSelector(url.Values{
				"source_id": sourceIDs,
				t:           {},
			})
			Expect(err).ToNot(HaveOccurred())

			Expect(s).To(Equal(selectors))
		},
		Entry("log", "log", []string{"app-1", "app-2"}, []*loggregator_v2.Selector{
			buildSelector("log", "app-1"),
			buildSelector("log", "app-2"),
		}),
		Entry("counter", "counter", []string{"app-1", "app-2"}, []*loggregator_v2.Selector{
			buildSelector("counter", "app-1"),
			buildSelector("counter", "app-2"),
		}),
		Entry("gauge", "gauge", []string{"app-1", "app-2"}, []*loggregator_v2.Selector{
			buildSelector("gauge", "app-1"),
			buildSelector("gauge", "app-2"),
		}),
		Entry("timer", "timer", []string{"app-1", "app-2"}, []*loggregator_v2.Selector{
			buildSelector("timer", "app-1"),
			buildSelector("timer", "app-2"),
		}),
		Entry("event", "event", []string{"app-1", "app-2"}, []*loggregator_v2.Selector{
			buildSelector("event", "app-1"),
			buildSelector("event", "app-2"),
		}),
	)

	It("includes selectors for all the given envelope types", func() {
		s, err := web.BuildSelector(url.Values{
			"log":     {},
			"counter": {},
			"gauge":   {},
			"timer":   {},
			"event":   {},
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(s).To(Equal([]*loggregator_v2.Selector{
			buildSelector("log", ""),
			buildSelector("counter", ""),
			buildSelector("gauge", ""),
			buildSelector("timer", ""),
			buildSelector("event", ""),
		}))
	})

	It("includes selectors with given counter names", func() {
		s, err := web.BuildSelector(url.Values{
			"counter.name": {"requestCount", "failures"},
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(s).To(Equal([]*loggregator_v2.Selector{
			buildCounterSelector("", "requestCount"),
			buildCounterSelector("", "failures"),
		}))
	})

	It("includes a selector with given gauge names", func() {
		s, err := web.BuildSelector(url.Values{
			"gauge.name": {"cpu,memory", "jvm.stuff"},
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(s).To(Equal([]*loggregator_v2.Selector{
			buildGaugeSelector("", []string{"cpu", "memory"}),
			buildGaugeSelector("", []string{"jvm.stuff"}),
		}))
	})

	It("returns an error if the url values is empty", func() {
		_, err := web.BuildSelector(url.Values{})
		Expect(err.Error()).To(MatchJSON(`{
			"error": "empty_query",
			"message": "query cannot be empty"
		}`))
	})

	It("returns an error if no types are given", func() {
		_, err := web.BuildSelector(url.Values{"source_id": {"app-1"}})
		Expect(err.Error()).To(MatchJSON(`{
			"error": "missing_envelope_type",
			"message": "query must provide at least one envelope type"
		}`))
	})

	It("returns an error when counter.name present but empty", func() {
		_, err := web.BuildSelector(url.Values{
			"counter.name": {},
		})
		Expect(err.Error()).To(MatchJSON(`{
			"error": "missing_counter_name",
			"message": "counter.name is invalid without value"
		}`))
	})

	It("returns an error when gauge.name present but empty", func() {
		_, err := web.BuildSelector(url.Values{
			"gauge.name": {},
		})
		Expect(err.Error()).To(MatchJSON(`{
			"error": "missing_gauge_name",
			"message": "gauge.name is invalid without value"
		}`))
	})
})

func buildSelector(envType, sourceID string) *loggregator_v2.Selector {
	s := &loggregator_v2.Selector{SourceId: sourceID}

	switch envType {
	case "log":
		s.Message = &loggregator_v2.Selector_Log{
			Log: &loggregator_v2.LogSelector{},
		}
	case "counter":
		s.Message = &loggregator_v2.Selector_Counter{
			Counter: &loggregator_v2.CounterSelector{},
		}
	case "gauge":
		s.Message = &loggregator_v2.Selector_Gauge{
			Gauge: &loggregator_v2.GaugeSelector{},
		}
	case "timer":
		s.Message = &loggregator_v2.Selector_Timer{
			Timer: &loggregator_v2.TimerSelector{},
		}
	case "event":
		s.Message = &loggregator_v2.Selector_Event{
			Event: &loggregator_v2.EventSelector{},
		}
	default:
		panic("unsupported envelope type")
	}

	return s
}

func buildCounterSelector(sourceID, name string) *loggregator_v2.Selector {
	return &loggregator_v2.Selector{
		SourceId: sourceID,
		Message: &loggregator_v2.Selector_Counter{
			Counter: &loggregator_v2.CounterSelector{
				Name: name,
			},
		},
	}
}

func buildGaugeSelector(sourceID string, names []string) *loggregator_v2.Selector {
	return &loggregator_v2.Selector{
		SourceId: sourceID,
		Message: &loggregator_v2.Selector_Gauge{
			Gauge: &loggregator_v2.GaugeSelector{
				Names: names,
			},
		},
	}
}
