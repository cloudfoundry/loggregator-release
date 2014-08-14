package varz_forwarder

import (
	"fmt"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"sync"
)

type VarzForwarder struct {
	metricsByOrigin map[string]metricsByName
	sync.RWMutex
}

type metricsByName map[string]uint64

func NewVarzForwarder() *VarzForwarder {
	return &VarzForwarder{
		metricsByOrigin: make(map[string]metricsByName),
	}
}

func (vf *VarzForwarder) Run(metricChan <-chan *events.Envelope, outputChan chan<- *events.Envelope) {
	for metric := range metricChan {
		if metric.GetEventType() == events.Envelope_ValueMetric {
			vf.addMetric(metric)
		}

		outputChan <- metric
	}
}

func (vf *VarzForwarder) Emit() instrumentation.Context {
	vf.RLock()
	defer vf.RUnlock()

	c := instrumentation.Context{Name: "forwarder"}
	metrics := []instrumentation.Metric{}

	for origin, originMetrics := range vf.metricsByOrigin {
		for name, value := range originMetrics {
			metricName := fmt.Sprintf("%s.%s", origin, name)
			metrics = append(metrics, instrumentation.Metric{Name: metricName, Value: value})
		}
	}

	c.Metrics = metrics
	return c
}

func (vf *VarzForwarder) addMetric(metric *events.Envelope) {
	vf.Lock()
	defer vf.Unlock()

	originMetrics, ok := vf.metricsByOrigin[metric.GetOrigin()]
	if !ok {
		vf.metricsByOrigin[metric.GetOrigin()] = make(metricsByName)
		originMetrics = vf.metricsByOrigin[metric.GetOrigin()]
	}

	originMetrics[metric.GetValueMetric().GetName()] = metric.GetValueMetric().GetValue()
}
