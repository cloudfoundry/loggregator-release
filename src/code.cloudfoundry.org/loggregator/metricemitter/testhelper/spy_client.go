package testhelper

import (
	"code.cloudfoundry.org/loggregator/metricemitter"
	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"
)

type counterMetric struct {
	metricName string
	metric     *metricemitter.Counter
}

type SpyMetricClient struct {
	counterMetrics []counterMetric
}

func NewMetricClient() *SpyMetricClient {
	return &SpyMetricClient{}
}

func (s *SpyMetricClient) NewCounter(name string, opts ...metricemitter.MetricOption) *metricemitter.Counter {
	m := metricemitter.NewCounter(name, "", opts...)

	s.counterMetrics = append(s.counterMetrics, counterMetric{
		metricName: name,
		metric:     m,
	})

	return m
}

func (s *SpyMetricClient) GetDelta(name string) uint64 {
	for _, m := range s.counterMetrics {
		if m.metricName == name {
			return m.metric.GetDelta()
		}
	}

	return 0
}

func (s *SpyMetricClient) GetEnvelopes(name string) []*v2.Envelope {
	var envs []*v2.Envelope

	for _, m := range s.counterMetrics {
		if m.metricName == name {
			var env *v2.Envelope
			m.metric.WithEnvelope(func(e *v2.Envelope) error {
				env = e
				return nil
			})

			envs = append(envs, env)
		}
	}

	return envs
}
