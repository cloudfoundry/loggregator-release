package web

import (
	"net/url"
	"strings"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
)

var (
	envelopeTypes = []string{"log", "counter", "gauge", "timer", "event"}
)

// BuildSelector returns a slice of v2 loggregator Selectors generated
// from the given query parameters
func BuildSelector(v url.Values) ([]*loggregator_v2.Selector, error) {
	if len(v) == 0 {
		return nil, errEmptyQuery
	}

	if !containsEnvelopeType(v) {
		return nil, errMissingType
	}

	if presentButEmpty(v, "counter.name") {
		return nil, errCounterNamePresentButEmpty
	}

	if presentButEmpty(v, "gauge.name") {
		return nil, errGaugeNamePresentButEmpty
	}

	sourceIDs := v["source_id"]
	if len(sourceIDs) == 0 {
		sourceIDs = []string{""}
	}

	var selectors []*loggregator_v2.Selector
	for _, sid := range sourceIDs {
		if hasKey(v, "log") {
			selectors = addLogSelector(selectors, sid)
		}

		if hasKey(v, "counter") || hasKey(v, "counter.name") {
			selectors = addCounterSelector(selectors, sid, v["counter.name"])
		}

		if hasKey(v, "gauge") || hasKey(v, "gauge.name") {
			selectors = addGaugeSelector(selectors, sid, v["gauge.name"])
		}

		if hasKey(v, "timer") {
			selectors = addTimerSelector(selectors, sid)
		}

		if hasKey(v, "event") {
			selectors = addEventSelector(selectors, sid)
		}
	}
	return selectors, nil
}

func hasKey(v url.Values, envType string) bool {
	_, ok := v[envType]
	return ok
}

func addLogSelector(
	selectors []*loggregator_v2.Selector,
	sourceID string,
) []*loggregator_v2.Selector {
	return append(selectors, &loggregator_v2.Selector{
		SourceId: sourceID,
		Message: &loggregator_v2.Selector_Log{
			Log: &loggregator_v2.LogSelector{},
		},
	})
}

func addCounterSelector(
	selectors []*loggregator_v2.Selector,
	sourceID string,
	names []string,
) []*loggregator_v2.Selector {

	if names == nil {
		return append(selectors, &loggregator_v2.Selector{
			SourceId: sourceID,
			Message: &loggregator_v2.Selector_Counter{
				Counter: &loggregator_v2.CounterSelector{},
			},
		})
	}

	for _, n := range names {
		selectors = append(selectors, &loggregator_v2.Selector{
			SourceId: sourceID,
			Message: &loggregator_v2.Selector_Counter{
				Counter: &loggregator_v2.CounterSelector{
					Name: n,
				},
			},
		})
	}

	return selectors
}

func addGaugeSelector(
	selectors []*loggregator_v2.Selector,
	sourceID string,
	names []string,
) []*loggregator_v2.Selector {
	if len(names) == 0 {
		return append(selectors, &loggregator_v2.Selector{
			SourceId: sourceID,
			Message: &loggregator_v2.Selector_Gauge{
				Gauge: &loggregator_v2.GaugeSelector{},
			},
		})
	}

	for _, name := range names {
		selectors = append(selectors, &loggregator_v2.Selector{
			SourceId: sourceID,
			Message: &loggregator_v2.Selector_Gauge{
				Gauge: &loggregator_v2.GaugeSelector{
					Names: strings.Split(name, ","),
				},
			},
		})
	}

	return selectors
}

func addTimerSelector(
	selectors []*loggregator_v2.Selector,
	sourceID string,
) []*loggregator_v2.Selector {
	return append(selectors, &loggregator_v2.Selector{
		SourceId: sourceID,
		Message: &loggregator_v2.Selector_Timer{
			Timer: &loggregator_v2.TimerSelector{},
		},
	})
}

func addEventSelector(
	selectors []*loggregator_v2.Selector,
	sourceID string,
) []*loggregator_v2.Selector {
	return append(selectors, &loggregator_v2.Selector{
		SourceId: sourceID,
		Message: &loggregator_v2.Selector_Event{
			Event: &loggregator_v2.EventSelector{},
		},
	})
}

func containsEnvelopeType(v url.Values) bool {
	if _, ok := v["counter.name"]; ok {
		return true
	}

	if _, ok := v["gauge.name"]; ok {
		return true
	}

	for _, t := range envelopeTypes {
		if _, ok := v[t]; ok {
			return true
		}
	}

	return false
}

func presentButEmpty(v url.Values, key string) bool {
	if n, ok := v[key]; ok && len(n) == 0 {
		return true
	}

	return false
}
