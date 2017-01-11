package conversion

import (
	"fmt"
	v2 "plumbing/v2"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

func ToV1(e *v2.Envelope) *events.Envelope {
	logMessage := e.GetLog()
	return &events.Envelope{
		Origin:     proto.String(e.Tags["origin"].GetText()),
		Deployment: proto.String(e.Tags["deployment"].GetText()),
		Job:        proto.String(e.Tags["job"].GetText()),
		Index:      proto.String(e.Tags["index"].GetText()),
		Timestamp:  proto.Int64(e.Timestamp),
		Ip:         proto.String(e.Tags["ip"].GetText()),
		EventType:  events.Envelope_LogMessage.Enum(),
		Tags:       convertTags(e.Tags),
		LogMessage: &events.LogMessage{
			Message:        logMessage.Payload,
			MessageType:    messageType(logMessage),
			Timestamp:      proto.Int64(e.Timestamp),
			AppId:          proto.String(e.SourceUuid),
			SourceType:     proto.String(e.Tags["source_type"].GetText()),
			SourceInstance: proto.String(e.Tags["source_instance"].GetText()),
		},
	}
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
