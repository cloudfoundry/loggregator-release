package app

import (
	"code.cloudfoundry.org/loggregator/metricemitter"
	"github.com/cloudfoundry/dropsonde/metrics"
)

// metricShim isolates all uses of the dropsonde metrics behind a clean
// interface. Any code which must send metrics should inject this shim.
// The shim exists for two reasons.
//
// First, the dropsonde metrics library relies on global state, which is prone
// to race conditions and is difficult to test reliably. The shim exists to be
// injected as an interface, which may be replaced with a double in test.
//
// Second, the shim exists to make replacing dropsonde metrics wholesale as
// simple as changing this file. If we know what interface we need for
// metrics, swapping implementations is much easier.
type metricShim struct {
	stream   *metricemitter.Counter
	firehose *metricemitter.Counter
}

func newMetricShim(client MetricClient) *metricShim {
	stream := client.NewCounter("egress", metricemitter.WithTags(
		map[string]string{"endpoint": "stream"},
	))
	firehose := client.NewCounter("egress", metricemitter.WithTags(
		map[string]string{"endpoint": "firehose"},
	))

	return &metricShim{
		stream:   stream,
		firehose: firehose,
	}
}

func (*metricShim) SendValue(name string, value float64, unit string) error {
	return metrics.SendValue(name, value, unit)
}

func (s *metricShim) IncrementEgressStream() {
	s.stream.Increment(1)
}

func (s *metricShim) IncrementEgressFirehose() {
	s.firehose.Increment(1)
}
