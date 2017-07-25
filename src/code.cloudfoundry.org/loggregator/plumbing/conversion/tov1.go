package conversion

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

// ToV1 converts v2 envelopes down to v1 envelopes.
func ToV1(e *v2.Envelope) []*events.Envelope {
	v1e := createBaseV1(e)

	switch (e.Message).(type) {
	case *v2.Envelope_Log:
		convertLog(v1e, e)
	case *v2.Envelope_Counter:
		convertCounter(v1e, e)
	case *v2.Envelope_Gauge:
		return convertGauge(e)
	case *v2.Envelope_Timer:
		convertTimer(v1e, e)
	default:
		return nil
	}

	return []*events.Envelope{v1e}
}

func createBaseV1(e *v2.Envelope) *events.Envelope {
	v1e := &events.Envelope{
		Origin:     proto.String(getV2Tag(e, "origin")),
		Deployment: proto.String(getV2Tag(e, "deployment")),
		Job:        proto.String(getV2Tag(e, "job")),
		Index:      proto.String(getV2Tag(e, "index")),
		Timestamp:  proto.Int64(e.Timestamp),
		Ip:         proto.String(getV2Tag(e, "ip")),
		Tags:       convertTags(e),
	}

	delete(v1e.Tags, "__v1_type")
	delete(v1e.Tags, "origin")
	delete(v1e.Tags, "deployment")
	delete(v1e.Tags, "job")
	delete(v1e.Tags, "index")
	delete(v1e.Tags, "ip")

	if e.SourceId != "" {
		v1e.Tags["source_id"] = e.SourceId
	}

	if e.InstanceId != "" {
		v1e.Tags["instance_id"] = e.InstanceId
	}

	return v1e
}

func getV2Tag(e *v2.Envelope, key string) string {
	if value, ok := e.GetTags()[key]; ok {
		return value
	}

	d := e.GetDeprecatedTags()[key]
	if d == nil {
		return ""
	}

	switch v := d.Data.(type) {
	case *v2.Value_Text:
		return v.Text
	case *v2.Value_Integer:
		return fmt.Sprintf("%d", v.Integer)
	case *v2.Value_Decimal:
		return fmt.Sprintf("%f", v.Decimal)
	default:
		return ""
	}
}

func convertTimer(v1e *events.Envelope, v2e *v2.Envelope) {
	timer := v2e.GetTimer()
	v1e.EventType = events.Envelope_HttpStartStop.Enum()

	method := events.Method(events.Method_value[getV2Tag(v2e, "method")])
	peerType := events.PeerType(events.PeerType_value[getV2Tag(v2e, "peer_type")])

	v1e.HttpStartStop = &events.HttpStartStop{
		StartTimestamp: proto.Int64(timer.Start),
		StopTimestamp:  proto.Int64(timer.Stop),
		RequestId:      convertUUID(parseUUID(getV2Tag(v2e, "request_id"))),
		ApplicationId:  convertUUID(parseUUID(v2e.SourceId)),
		PeerType:       &peerType,
		Method:         &method,
		Uri:            proto.String(getV2Tag(v2e, "uri")),
		RemoteAddress:  proto.String(getV2Tag(v2e, "remote_address")),
		UserAgent:      proto.String(getV2Tag(v2e, "user_agent")),
		StatusCode:     proto.Int32(int32(atoi(getV2Tag(v2e, "status_code")))),
		ContentLength:  proto.Int64(atoi(getV2Tag(v2e, "content_length"))),
		InstanceIndex:  proto.Int32(int32(atoi(getV2Tag(v2e, "instance_index")))),
		InstanceId:     proto.String(getV2Tag(v2e, "routing_instance_id")),
		Forwarded:      strings.Split(getV2Tag(v2e, "forwarded"), "\n"),
	}

	delete(v1e.Tags, "peer_type")
	delete(v1e.Tags, "method")
	delete(v1e.Tags, "request_id")
	delete(v1e.Tags, "uri")
	delete(v1e.Tags, "remote_address")
	delete(v1e.Tags, "user_agent")
	delete(v1e.Tags, "status_code")
	delete(v1e.Tags, "content_length")
	delete(v1e.Tags, "instance_index")
	delete(v1e.Tags, "routing_instance_id")
	delete(v1e.Tags, "forwarded")
}

func atoi(s string) int64 {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}

	return i
}

func convertLog(v1e *events.Envelope, v2e *v2.Envelope) {
	if getV2Tag(v2e, "__v1_type") == "Error" {
		recoverError(v1e, v2e)
		return
	}
	logMessage := v2e.GetLog()
	v1e.EventType = events.Envelope_LogMessage.Enum()
	v1e.LogMessage = &events.LogMessage{
		Message:        logMessage.Payload,
		MessageType:    messageType(logMessage),
		Timestamp:      proto.Int64(v2e.Timestamp),
		AppId:          proto.String(v2e.SourceId),
		SourceType:     proto.String(getV2Tag(v2e, "source_type")),
		SourceInstance: proto.String(v2e.InstanceId),
	}
	delete(v1e.Tags, "source_type")
}

func recoverError(v1e *events.Envelope, v2e *v2.Envelope) {
	logMessage := v2e.GetLog()
	v1e.EventType = events.Envelope_Error.Enum()
	code := int32(atoi(getV2Tag(v2e, "code")))
	v1e.Error = &events.Error{
		Source:  proto.String(getV2Tag(v2e, "source")),
		Code:    proto.Int32(code),
		Message: proto.String(string(logMessage.Payload)),
	}
	delete(v1e.Tags, "source")
	delete(v1e.Tags, "code")
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

func convertGauge(v2e *v2.Envelope) []*events.Envelope {
	if v1e := tryConvertContainerMetric(v2e); v1e != nil {
		return []*events.Envelope{v1e}
	}

	var results []*events.Envelope
	gaugeEvent := v2e.GetGauge()

	for key, metric := range gaugeEvent.Metrics {
		v1e := createBaseV1(v2e)
		v1e.EventType = events.Envelope_ValueMetric.Enum()
		unit, value, ok := extractGaugeValues(metric)
		if !ok {
			return nil
		}

		v1e.ValueMetric = &events.ValueMetric{
			Name:  proto.String(key),
			Unit:  proto.String(unit),
			Value: proto.Float64(value),
		}
		results = append(results, v1e)
	}

	return results
}

func extractGaugeValues(metric *v2.GaugeValue) (string, float64, bool) {
	if metric == nil {
		return "", 0, false
	}

	return metric.Unit, metric.Value, true
}

func tryConvertContainerMetric(v2e *v2.Envelope) *events.Envelope {
	v1e := createBaseV1(v2e)
	gaugeEvent := v2e.GetGauge()
	if len(gaugeEvent.Metrics) == 1 {
		return nil
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
			return nil
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

	return v1e
}

func convertTags(e *v2.Envelope) map[string]string {
	oldTags := e.Tags
	if oldTags == nil {
		oldTags = make(map[string]string)
	}

	for key, value := range e.GetDeprecatedTags() {
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
