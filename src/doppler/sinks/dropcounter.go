package sinks
import "sync/atomic"

type DropCounter struct {
    droppedMessageCount int64
    appID string
    drainURL string
}

func NewDropCounter(appID string, drainURL string) DropCounter {
    return DropCounter{appID: appID, drainURL: drainURL}
}

func (dc *DropCounter) GetInstrumentationMetric() Metric {
    count := atomic.LoadInt64(&dc.droppedMessageCount)
    return Metric{Name: "numberOfMessagesLost", Tags: map[string]interface{}{"appId": dc.appID, "drainUrl": dc.drainURL}, Value: count}
}

func (dc *DropCounter) UpdateDroppedMessageCount(messageCount int64) {
    atomic.AddInt64(&dc.droppedMessageCount, messageCount)
}