package varz_forwarder

import (
	"fmt"
	"sync"
	"time"

	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
)

type VarzForwarder struct {
	metricsByOrigin map[string]*metrics
	componentName   string
	ttl             time.Duration
	logger          *gosteno.Logger
	sync.RWMutex
}

func New(componentName string, ttl time.Duration, logger *gosteno.Logger) *VarzForwarder {
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

func (vf *VarzForwarder) addMetric(metric *events.Envelope) {
	vf.Lock()
	defer vf.Unlock()

	originMetrics, ok := vf.metricsByOrigin[metric.GetOrigin()]
	if !ok {
		vf.metricsByOrigin[metric.GetOrigin()] = vf.createMetrics(metric.GetOrigin())
		originMetrics = vf.metricsByOrigin[metric.GetOrigin()]
	}

	originMetrics.ProcessMetric(metric)
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

type metrics struct {
	metricsByName map[string]float64
	timer         *time.Timer
}

func (metrics *metrics) ProcessMetric(metric *events.Envelope) {
	switch metric.GetEventType() {
	case events.Envelope_ValueMetric:
		metrics.processValueMetric(metric)
	case events.Envelope_CounterEvent:
		metrics.processCounterEvent(metric)
	case events.Envelope_HttpStartStop:
		metrics.processHttpStartStop(metric)
	}
}

func (metrics *metrics) processValueMetric(metric *events.Envelope) {
	metrics.metricsByName[metric.GetValueMetric().GetName()] = metric.GetValueMetric().GetValue()
}

func (metrics *metrics) processCounterEvent(metric *events.Envelope) {
	eventName := metric.GetCounterEvent().GetName()
	count := metrics.metricsByName[eventName]
	metrics.metricsByName[eventName] = count + float64(metric.GetCounterEvent().GetDelta())
}

func (metrics *metrics) processHttpStartStop(metric *events.Envelope) {
	eventName := "requestCount"
	count := metrics.metricsByName[eventName]
	metrics.metricsByName[eventName] = count + 1

	startStop := metric.GetHttpStartStop()
	status := startStop.GetStatusCode()
	switch {
	case status >= 100 && status < 200:
		metrics.metricsByName["responseCount1XX"] = metrics.metricsByName["responseCount1XX"] + 1
	case status >= 200 && status < 300:
		metrics.metricsByName["responseCount2XX"] = metrics.metricsByName["responseCount2XX"] + 1
	case status >= 300 && status < 400:
		metrics.metricsByName["responseCount3XX"] = metrics.metricsByName["responseCount3XX"] + 1
	case status >= 400 && status < 500:
		metrics.metricsByName["responseCount4XX"] = metrics.metricsByName["responseCount4XX"] + 1
	case status >= 500 && status < 600:
		metrics.metricsByName["responseCount5XX"] = metrics.metricsByName["responseCount5XX"] + 1
	default:
	}
}
