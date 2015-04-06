package sinks

import "sync/atomic"

type DropCounter struct {
	droppedMessageCount int64
	appID               string
	drainURL            string
	metricUpdateChannel chan<- int64
}

func NewDropCounter(appID string, drainURL string, metricUpdateChannel chan<- int64) DropCounter {
	return DropCounter{appID: appID, drainURL: drainURL, metricUpdateChannel: metricUpdateChannel}
}

func (dc *DropCounter) GetInstrumentationMetric() Metric {
	count := atomic.LoadInt64(&dc.droppedMessageCount)
	return Metric{Name: "numberOfMessagesLost", Tags: map[string]interface{}{"appId": dc.appID, "drainUrl": dc.drainURL}, Value: count}
}

func (dc *DropCounter) UpdateDroppedMessageCount(messageCount int64) {
	atomic.AddInt64(&dc.droppedMessageCount, messageCount)
	dc.metricUpdateChannel <- messageCount
}
