package testhelper

import "metricemitter"

type SpyMetricClient struct{}

func NewMetricClient() *SpyMetricClient {
	return &SpyMetricClient{}
}

func (s *SpyMetricClient) NewCounterMetric(name string, opts ...metricemitter.MetricOption) *metricemitter.CounterMetric {
	return &metricemitter.CounterMetric{}
}
