package varz_forwarder

import (
	"time"

	"github.com/cloudfoundry/dropsonde/events"
)

type metrics struct {
	metricsByName map[string]float64
	timer         *time.Timer
}

func (metrics *metrics) processMetric(metric *events.Envelope) {
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
