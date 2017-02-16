package conversion

import (
	"encoding/binary"
	"fmt"
	v2 "plumbing/v2"
	"strings"

	"github.com/cloudfoundry/sonde-go/events"
)

// ToV2 converts v1 envelopes up to v2 envelopes.
func ToV2(e *events.Envelope) *v2.Envelope {
	v2e := &v2.Envelope{
		Timestamp: e.GetTimestamp(),
		Tags:      buildTags(e.GetTags()),
	}
	v2e.Tags["origin"] = valueText(e.GetOrigin())

	switch e.GetEventType() {
	case events.Envelope_LogMessage:
		convertLogMessage(v2e, e)
	case events.Envelope_HttpStartStop:
		convertHTTPStartStop(v2e, e)
	case events.Envelope_ValueMetric:
		convertValueMetric(v2e, e)
	case events.Envelope_CounterEvent:
		convertCounterEvent(v2e, e)
	case events.Envelope_Error:
		convertError(v2e, e)
	case events.Envelope_ContainerMetric:
		convertContainerMetric(v2e, e)
	}

	return v2e
}

func buildTags(oldTags map[string]string) map[string]*v2.Value {
	newTags := make(map[string]*v2.Value)
	for k, v := range oldTags {
		newTags[k] = valueText(v)
	}
	return newTags
}

func convertError(v2e *v2.Envelope, v1e *events.Envelope) {
	t := v1e.GetError()
	v2e.Tags["source"] = valueText(t.GetSource())
	v2e.Tags["code"] = valueInt32(t.GetCode())

	v2e.Message = &v2.Envelope_Log{
		Log: &v2.Log{
			Payload: []byte(t.GetMessage()),
			Type:    v2.Log_OUT,
		},
	}
}

func convertHTTPStartStop(v2e *v2.Envelope, v1e *events.Envelope) {
	t := v1e.GetHttpStartStop()
	v2e.SourceId = uuidToString(t.GetApplicationId())
	v2e.Message = &v2.Envelope_Timer{
		Timer: &v2.Timer{
			Name:  "http",
			Start: t.GetStartTimestamp(),
			Stop:  t.GetStopTimestamp(),
		},
	}
	v2e.Tags["request_id"] = valueText(uuidToString(t.GetRequestId()))
	v2e.Tags["peer_type"] = valueText(t.GetPeerType().String())
	v2e.Tags["method"] = valueText(t.GetMethod().String())
	v2e.Tags["uri"] = valueText(t.GetUri())
	v2e.Tags["remote_address"] = valueText(t.GetRemoteAddress())
	v2e.Tags["user_agent"] = valueText(t.GetUserAgent())
	v2e.Tags["status_code"] = valueInt32(t.GetStatusCode())
	v2e.Tags["content_length"] = valueInt64(t.GetContentLength())
	v2e.Tags["instance_index"] = valueInt32(t.GetInstanceIndex())
	v2e.Tags["instance_id"] = valueText(t.GetInstanceId())
	v2e.Tags["forwarded"] = valueTextSlice(t.GetForwarded())
}

func convertLogMessageType(t events.LogMessage_MessageType) v2.Log_Type {
	name := events.LogMessage_MessageType_name[int32(t)]
	return v2.Log_Type(v2.Log_Type_value[name])
}

func convertLogMessage(v2e *v2.Envelope, e *events.Envelope) {
	t := e.GetLogMessage()
	v2e.Tags["source_type"] = valueText(t.GetSourceType())
	v2e.Tags["source_instance"] = valueText(t.GetSourceInstance())

	v2e.SourceId = t.GetAppId()
	v2e.Message = &v2.Envelope_Log{
		Log: &v2.Log{
			Payload: t.GetMessage(),
			Type:    convertLogMessageType(t.GetMessageType()),
		},
	}
}

func convertValueMetric(v2e *v2.Envelope, e *events.Envelope) {
	t := e.GetValueMetric()
	v2e.Message = &v2.Envelope_Gauge{
		Gauge: &v2.Gauge{
			Metrics: map[string]*v2.GaugeValue{
				t.GetName(): {
					Unit:  t.GetUnit(),
					Value: t.GetValue(),
				},
			},
		},
	}
}

func convertCounterEvent(v2e *v2.Envelope, e *events.Envelope) {
	t := e.GetCounterEvent()
	v2e.Message = &v2.Envelope_Counter{
		Counter: &v2.Counter{
			Name: t.GetName(),
			Value: &v2.Counter_Total{
				Total: t.GetTotal(),
			},
		},
	}
}

func convertContainerMetric(v2e *v2.Envelope, e *events.Envelope) {
	t := e.GetContainerMetric()
	v2e.SourceId = t.GetApplicationId()
	v2e.Message = &v2.Envelope_Gauge{
		Gauge: &v2.Gauge{
			Metrics: map[string]*v2.GaugeValue{
				"instance_index": {
					Unit:  "index",
					Value: float64(t.GetInstanceIndex()),
				},
				"cpu": {
					Unit:  "percentage",
					Value: float64(t.GetCpuPercentage()),
				},
				"memory": {
					Unit:  "bytes",
					Value: float64(t.GetMemoryBytes()),
				},
				"disk": {
					Unit:  "bytes",
					Value: float64(t.GetDiskBytes()),
				},
				"memory_quota": {
					Unit:  "bytes",
					Value: float64(t.GetMemoryBytesQuota()),
				},
				"disk_quota": {
					Unit:  "bytes",
					Value: float64(t.GetDiskBytesQuota()),
				},
			},
		},
	}
}

func valueText(s string) *v2.Value {
	return &v2.Value{&v2.Value_Text{Text: s}}
}

func valueInt64(i int64) *v2.Value {
	return &v2.Value{&v2.Value_Integer{Integer: i}}
}

func valueInt32(i int32) *v2.Value {
	return &v2.Value{&v2.Value_Integer{Integer: int64(i)}}
}

func valueTextSlice(s []string) *v2.Value {
	text := strings.Join(s, "\n")
	return &v2.Value{&v2.Value_Text{Text: text}}
}

func uuidToString(uuid *events.UUID) string {
	low := make([]byte, 8)
	high := make([]byte, 8)
	binary.LittleEndian.PutUint64(low, uuid.GetLow())
	binary.LittleEndian.PutUint64(high, uuid.GetHigh())
	return fmt.Sprintf("%x-%x-%x-%x-%x", low[:4], low[4:6], low[6:], high[:2], high[2:])
}
