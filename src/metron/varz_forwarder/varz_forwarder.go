package varz_forwarder

import (
	"fmt"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"sync"
)

type VarzForwarder struct {
	metricsByOrigin map[string]metricsByName
	componentName   string
	sync.RWMutex
}

type metricsByName map[string]float64

func NewVarzForwarder(componentName string) *VarzForwarder {
	return &VarzForwarder{
		metricsByOrigin: make(map[string]metricsByName),
		componentName:   componentName,
	}
}

func (vf *VarzForwarder) Run(metricChan <-chan *events.Envelope, outputChan chan<- *events.Envelope) {
	for metric := range metricChan {
		vf.addMetric(metric)

		outputChan <- metric
	}
}

func (vf *VarzForwarder) addMetric(metric *events.Envelope) {
	metricProcessor, ok := metricProcessorsByType[metric.GetEventType()]
	if !ok {
		return
	}

	vf.Lock()
	defer vf.Unlock()

	originMetrics, ok := vf.metricsByOrigin[metric.GetOrigin()]
	if !ok {
		vf.metricsByOrigin[metric.GetOrigin()] = make(metricsByName)
		originMetrics = vf.metricsByOrigin[metric.GetOrigin()]
	}

	metricProcessor(originMetrics, metric)
}

func (vf *VarzForwarder) Emit() instrumentation.Context {
	vf.RLock()
	defer vf.RUnlock()

	c := instrumentation.Context{Name: "forwarder"}
	metrics := []instrumentation.Metric{}
	tags := map[string]interface{}{
		"component": vf.componentName,
	}

	for origin, originMetrics := range vf.metricsByOrigin {
		for name, value := range originMetrics {
			metricName := fmt.Sprintf("%s.%s", origin, name)
			metrics = append(metrics, instrumentation.Metric{Name: metricName, Value: value, Tags: tags})
		}
	}

	c.Metrics = metrics
	return c
}

var metricProcessorsByType = map[events.Envelope_EventType]func(metricsByName, *events.Envelope){
	events.Envelope_ValueMetric:  metricsByName.processValueMetric,
	events.Envelope_CounterEvent: metricsByName.processCounterEvent,
}

func (metrics metricsByName) processValueMetric(metric *events.Envelope) {
	metrics[metric.GetValueMetric().GetName()] = metric.GetValueMetric().GetValue()
}

func (metrics metricsByName) processCounterEvent(metric *events.Envelope) {
	eventName := metric.GetCounterEvent().GetName()
	count, ok := metrics[eventName]

	if !ok {
		metrics[eventName] = 1
	} else {
		metrics[eventName] = count + 1
	}
}
