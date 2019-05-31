package sinks

import (
	"code.cloudfoundry.org/loggregator/metricemitter"
)

type SinkManagerMetrics struct {
	dumpSinksMetric *metricemitter.Gauge
}

func NewSinkManagerMetrics(mc MetricClient) *SinkManagerMetrics {
	// metric-documentation-v2: (loggregator.doppler.dump_sinks) Number of
	// recent log sinks.
	dumpSinksMetric := mc.NewGauge("dump_sinks", "sinks",
		metricemitter.WithVersion(2, 0),
	)

	return &SinkManagerMetrics{
		dumpSinksMetric: dumpSinksMetric,
	}
}

func (s *SinkManagerMetrics) Inc(sink Sink) {
	switch sink.(type) {
	case *DumpSink:
		s.dumpSinksMetric.Increment(1.0)
	}
}

func (s *SinkManagerMetrics) Dec(sink Sink) {
	switch sink.(type) {
	case *DumpSink:
		s.dumpSinksMetric.Decrement(1.0)
	}
}
