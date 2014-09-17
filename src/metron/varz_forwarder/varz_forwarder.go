package varz_forwarder

import (
	"fmt"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"sync"
	"time"
)

type VarzForwarder struct {
	metricsByOrigin map[string]*metrics
	componentName   string
	ttl             time.Duration
	logger          *gosteno.Logger
	sync.RWMutex
}

type metrics struct {
	metricsByName map[string]float64
	timer         *time.Timer
	sync.RWMutex
}

func NewVarzForwarder(componentName string, ttl time.Duration, logger *gosteno.Logger) *VarzForwarder {
	return &VarzForwarder{
		metricsByOrigin: make(map[string]*metrics),
		componentName:   componentName,
		ttl:             ttl,
		logger:          logger,
	}
}

func (vf *VarzForwarder) Run(metricChan <-chan *events.Envelope, outputChan chan<- *events.Envelope) {
	for metric := range metricChan {
		vf.addMetric(metric)
		vf.resetTimer(metric.GetOrigin())

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
		vf.metricsByOrigin[metric.GetOrigin()] = vf.createMetrics(metric.GetOrigin())
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
		for name, value := range originMetrics.metricsByName {
			metricName := fmt.Sprintf("%s.%s", origin, name)
			metrics = append(metrics, instrumentation.Metric{Name: metricName, Value: value, Tags: tags})
		}
	}

	c.Metrics = metrics
	return c
}

func (vf *VarzForwarder) createMetrics(origin string) *metrics {
	vf.logger.Debugf("creating metrics for origin %v", origin)
	return &metrics{
		metricsByName: make(map[string]float64),
		timer:         time.AfterFunc(vf.ttl, func() { vf.deleteMetrics(origin) }),
	}
}

func (vf *VarzForwarder) deleteMetrics(origin string) {
	vf.logger.Debugf("deleting metrics for origin %v", origin)
	vf.Lock()
	defer vf.Unlock()

	delete(vf.metricsByOrigin, origin)
}

func (vf *VarzForwarder) resetTimer(origin string) {
	vf.RLock()
	defer vf.RUnlock()

	metrics, ok := vf.metricsByOrigin[origin]
	if ok {
		metrics.timer.Reset(vf.ttl)
	}
}

var metricProcessorsByType = map[events.Envelope_EventType]func(*metrics, *events.Envelope){
	events.Envelope_ValueMetric:  (*metrics).processValueMetric,
	events.Envelope_CounterEvent: (*metrics).processCounterEvent,
}

func (metrics *metrics) processValueMetric(metric *events.Envelope) {
	metrics.RLock()
	defer metrics.RUnlock()

	metrics.metricsByName[metric.GetValueMetric().GetName()] = metric.GetValueMetric().GetValue()
}

func (metrics *metrics) processCounterEvent(metric *events.Envelope) {
	metrics.Lock()
	defer metrics.Unlock()

	eventName := metric.GetCounterEvent().GetName()
	count, ok := metrics.metricsByName[eventName]

	if !ok {
		metrics.metricsByName[eventName] = 1
	} else {
		metrics.metricsByName[eventName] = count + 1
	}
}
