package conversion

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	v2 "plumbing/v2"
	"strings"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

// ToV1 converts v2 envelopes down to v1 envelopes.
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
		if !convertGauge(v1e, e) {
			return nil
		}
	case *v2.Envelope_Timer:
		convertTimer(v1e, e)
	default:
		return nil
	}

	return v1e
}

func convertTimer(v1e *events.Envelope, v2e *v2.Envelope) {
	timer := v2e.GetTimer()
	v1e.EventType = events.Envelope_HttpStartStop.Enum()

	method := events.Method(events.Method_value[v2e.Tags["method"].GetText()])
	peerType := events.PeerType(events.PeerType_value[v2e.Tags["peer_type"].GetText()])

	v1e.HttpStartStop = &events.HttpStartStop{
		StartTimestamp: proto.Int64(timer.Start),
		StopTimestamp:  proto.Int64(timer.Stop),
		RequestId:      convertUUID(parseUUID(v2e.Tags["request_id"].GetText())),
		ApplicationId:  convertUUID(parseUUID(v2e.SourceId)),
		PeerType:       &peerType,
		Method:         &method,
		Uri:            proto.String(v2e.Tags["uri"].GetText()),
		RemoteAddress:  proto.String(v2e.Tags["remote_address"].GetText()),
		UserAgent:      proto.String(v2e.Tags["user_agent"].GetText()),
		StatusCode:     proto.Int32(int32(v2e.Tags["status_code"].GetInteger())),
		ContentLength:  proto.Int64(v2e.Tags["content_length"].GetInteger()),
		InstanceIndex:  proto.Int32(int32(v2e.Tags["instance_index"].GetInteger())),
		InstanceId:     proto.String(v2e.Tags["instance_id"].GetText()),
		Forwarded:      strings.Split(v2e.Tags["forwarded"].GetText(), "\n"),
	}
}

func convertLog(v1e *events.Envelope, v2e *v2.Envelope) {
	logMessage := v2e.GetLog()
	v1e.EventType = events.Envelope_LogMessage.Enum()
	v1e.LogMessage = &events.LogMessage{
		Message:        logMessage.Payload,
		MessageType:    messageType(logMessage),
		Timestamp:      proto.Int64(v2e.Timestamp),
		AppId:          proto.String(v2e.SourceId),
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

func convertGauge(v1e *events.Envelope, v2e *v2.Envelope) bool {
	if tryConvertContainerMetric(v1e, v2e) {
		return true
	}

	gaugeEvent := v2e.GetGauge()
	if len(gaugeEvent.Metrics) != 1 {
		return false
	}

	v1e.EventType = events.Envelope_ValueMetric.Enum()
	for key, metric := range gaugeEvent.Metrics {
		unit, value, ok := extractGaugeValues(metric)
		if !ok {
			return false
		}

		v1e.ValueMetric = &events.ValueMetric{
			Name:  proto.String(key),
			Unit:  proto.String(unit),
			Value: proto.Float64(value),
		}
		return true
	}

	return false
}

func extractGaugeValues(metric *v2.GaugeValue) (string, float64, bool) {
	if metric == nil {
		return "", 0, false
	}

	return metric.Unit, metric.Value, true
}

func tryConvertContainerMetric(v1e *events.Envelope, v2e *v2.Envelope) bool {
	gaugeEvent := v2e.GetGauge()
	if len(gaugeEvent.Metrics) == 1 {
		return false
	}

	required := []string{
		"instance_index",
		"cpu",
		"memory",
		"disk",
		"memory_quota",
		"disk_quota",
	}

	for _, req := range required {
		if v, ok := gaugeEvent.Metrics[req]; !ok || v == nil {
			return false
		}
	}

	v1e.EventType = events.Envelope_ContainerMetric.Enum()
	v1e.ContainerMetric = &events.ContainerMetric{
		ApplicationId:    proto.String(v2e.SourceId),
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
		if value == nil {
			continue
		}
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

func parseUUID(id string) []byte {
	// e.g. b3015d69-09cd-476d-aace-ad2d824d5ab7
	if len(id) != 36 {
		return nil
	}
	h := id[:8] + id[9:13] + id[14:18] + id[19:23] + id[24:]

	data, err := hex.DecodeString(h)
	if err != nil {
		return nil
	}

	return data
}

func convertUUID(id []byte) *events.UUID {
	if len(id) != 16 {
		return &events.UUID{}
	}

	return &events.UUID{
		Low:  proto.Uint64(binary.LittleEndian.Uint64(id[:8])),
		High: proto.Uint64(binary.LittleEndian.Uint64(id[8:])),
	}
}
