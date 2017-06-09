package testhelper

import "code.cloudfoundry.org/loggregator/metricemitter"

type SpyMetricClient struct {
	counterMetrics map[string]*metricemitter.CounterMetric
}

func NewMetricClient() *SpyMetricClient {
	return &SpyMetricClient{
		counterMetrics: make(map[string]*metricemitter.CounterMetric),
	}
}

func (s *SpyMetricClient) NewCounterMetric(name string, opts ...metricemitter.MetricOption) *metricemitter.CounterMetric {
	m := &metricemitter.CounterMetric{}
	s.counterMetrics[name] = m

	return m
}

func (s *SpyMetricClient) GetDelta(name string) uint64 {
	return s.counterMetrics[name].GetDelta()
}
