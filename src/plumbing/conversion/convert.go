package conversion

import (
	"fmt"
	v2 "plumbing/v2"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

func ToV1(e *v2.Envelope) *events.Envelope {
	v1e := &events.Envelope{
		Origin:     proto.String(e.Tags["origin"].GetText()),
		Deployment: proto.String(e.Tags["deployment"].GetText()),
		Job:        proto.String(e.Tags["job"].GetText()),
		Index:      proto.String(e.Tags["index"].GetText()),
		Timestamp:  proto.Int64(e.Timestamp),
		Ip:         proto.String(e.Tags["ip"].GetText()),
		Tags:       convertTags(e.Tags),
	}

	switch (e.Message).(type) {
	case *v2.Envelope_Log:
		convertLog(v1e, e)
	case *v2.Envelope_Counter:
		convertCounter(v1e, e)
	case *v2.Envelope_Gauge:
		convertGauge(v1e, e)
	}

	return v1e
}

func convertLog(v1e *events.Envelope, v2e *v2.Envelope) {
	logMessage := v2e.GetLog()
	v1e.EventType = events.Envelope_LogMessage.Enum()
	v1e.LogMessage = &events.LogMessage{
		Message:        logMessage.Payload,
		MessageType:    messageType(logMessage),
		Timestamp:      proto.Int64(v2e.Timestamp),
		AppId:          proto.String(v2e.SourceUuid),
		SourceType:     proto.String(v2e.Tags["source_type"].GetText()),
		SourceInstance: proto.String(v2e.Tags["source_instance"].GetText()),
	}
}

func convertCounter(v1e *events.Envelope, v2e *v2.Envelope) {
	counterEvent := v2e.GetCounter()
	v1e.EventType = events.Envelope_CounterEvent.Enum()
	v1e.CounterEvent = &events.CounterEvent{
		Name:  proto.String(counterEvent.Name),
		Delta: proto.Uint64(0),
		Total: proto.Uint64(counterEvent.GetTotal()),
	}
}

func convertGauge(v1e *events.Envelope, v2e *v2.Envelope) {
	if tryConvertContainerMetric(v1e, v2e) {
		return
	}

	gaugeEvent := v2e.GetGauge()
	v1e.EventType = events.Envelope_ValueMetric.Enum()
	for key, metric := range gaugeEvent.Metrics {
		v1e.ValueMetric = &events.ValueMetric{
			Name:  proto.String(key),
			Unit:  proto.String(metric.Unit),
			Value: proto.Float64(metric.Value),
		}
		break
	}
}

func tryConvertContainerMetric(v1e *events.Envelope, v2e *v2.Envelope) bool {
	gaugeEvent := v2e.GetGauge()
	if len(gaugeEvent.Metrics) == 1 {
		return false
	}

	v1e.EventType = events.Envelope_ContainerMetric.Enum()
	v1e.ContainerMetric = &events.ContainerMetric{
		ApplicationId:    proto.String(v2e.SourceUuid),
		InstanceIndex:    proto.Int32(int32(gaugeEvent.Metrics["instance_index"].Value)),
		CpuPercentage:    proto.Float64(gaugeEvent.Metrics["cpu"].Value),
		MemoryBytes:      proto.Uint64(uint64(gaugeEvent.Metrics["memory"].Value)),
		DiskBytes:        proto.Uint64(uint64(gaugeEvent.Metrics["disk"].Value)),
		MemoryBytesQuota: proto.Uint64(uint64(gaugeEvent.Metrics["memory_quota"].Value)),
		DiskBytesQuota:   proto.Uint64(uint64(gaugeEvent.Metrics["disk_quota"].Value)),
	}

	return true
}

func convertTags(tags map[string]*v2.Value) map[string]string {
	oldTags := make(map[string]string)
	for key, value := range tags {
		switch value.Data.(type) {
		case *v2.Value_Text:
			oldTags[key] = value.GetText()
		case *v2.Value_Integer:
			oldTags[key] = fmt.Sprintf("%d", value.GetInteger())
		case *v2.Value_Decimal:
			oldTags[key] = fmt.Sprintf("%f", value.GetDecimal())
		}
	}
	return oldTags
}

func messageType(log *v2.Log) *events.LogMessage_MessageType {
	if log.Type == v2.Log_OUT {
		return events.LogMessage_OUT.Enum()
	}
	return events.LogMessage_ERR.Enum()
}
