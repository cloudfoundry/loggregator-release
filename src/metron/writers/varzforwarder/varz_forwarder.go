package varzforwarder

import (
	"fmt"
	"sync"
	"time"

	"metron/writers"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/sonde-go/events"
)

type VarzForwarder struct {
	metricsByOrigin map[string]*metrics
	componentName   string
	ttl             time.Duration

	logger       *gosteno.Logger
	outputWriter writers.EnvelopeWriter
	lock         sync.RWMutex
}

func New(componentName string, ttl time.Duration, logger *gosteno.Logger, outputWriter writers.EnvelopeWriter) *VarzForwarder {
	return &VarzForwarder{
		metricsByOrigin: make(map[string]*metrics),
		componentName:   componentName,
		ttl:             ttl,
		logger:          logger,
		outputWriter:    outputWriter,
	}
}

func (vf *VarzForwarder) Write(envelope *events.Envelope) {
	vf.addMetric(envelope)
	vf.resetTimer(envelope.GetOrigin())

	vf.outputWriter.Write(envelope)
}

func (vf *VarzForwarder) Emit() instrumentation.Context {
	vf.lock.RLock()
	defer vf.lock.RUnlock()

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
	vf.lock.Lock()
	defer vf.lock.Unlock()

	originMetrics, ok := vf.metricsByOrigin[metric.GetOrigin()]
	if !ok {
		vf.metricsByOrigin[metric.GetOrigin()] = vf.createMetrics(metric.GetOrigin())
		originMetrics = vf.metricsByOrigin[metric.GetOrigin()]
	}

	originMetrics.processMetric(metric)
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
	vf.lock.Lock()
	defer vf.lock.Unlock()

	delete(vf.metricsByOrigin, origin)
}

func (vf *VarzForwarder) resetTimer(origin string) {
	vf.lock.RLock()
	defer vf.lock.RUnlock()

	metrics, ok := vf.metricsByOrigin[origin]
	if ok {
		metrics.timer.Reset(vf.ttl)
	}
}
