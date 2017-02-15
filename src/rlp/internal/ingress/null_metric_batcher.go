package ingress

import "github.com/cloudfoundry/dropsonde/metricbatcher"

type NullMetricBatcher struct{}

func (n *NullMetricBatcher) BatchCounter(string) metricbatcher.BatchCounterChainer {
	return n
}
func (n *NullMetricBatcher) BatchAddCounter(string, uint64) {}
func (n *NullMetricBatcher) SetTag(string, string) metricbatcher.BatchCounterChainer {
	return n
}
func (n *NullMetricBatcher) Increment() {}
func (n *NullMetricBatcher) Add(uint64) {}
