package conversion_test

import (
	. "plumbing/conversion"
	"time"

	v2 "plumbing/v2"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = DescribeTable("v1->v2->v1",
	func(v1e *events.Envelope) {
		_, err := proto.Marshal(v1e)
		Expect(err).ToNot(HaveOccurred())

		v2e := ToV2(v1e)

		_, err = proto.Marshal(v2e)
		Expect(err).ToNot(HaveOccurred())

		newV1e := ToV1(v2e)
		Expect(newV1e).To(Equal(v1e))
	},
	Entry("HttpStartStop", &events.Envelope{
		Origin:     proto.String("some-origin"),
		Timestamp:  proto.Int64(1234),
		Deployment: proto.String("test-deployment"),
		Job:        proto.String("test-job"),
		Index:      proto.String("test-index"),
		Ip:         proto.String("test-ip"),
		Tags: map[string]string{
			"some-random": "tag",
			"source_id":   "08000000-0000-0000-0500-000000000000",
		},
		EventType: events.Envelope_HttpStartStop.Enum(),
		HttpStartStop: &events.HttpStartStop{
			StartTimestamp: proto.Int64(1),
			StopTimestamp:  proto.Int64(2),
			RequestId: &events.UUID{
				High: proto.Uint64(3),
				Low:  proto.Uint64(4),
			},
			PeerType:      events.PeerType_Server.Enum(),
			Method:        events.Method_PUT.Enum(),
			Uri:           proto.String("http://example.com"),
			RemoteAddress: proto.String("0.0.0.0"),
			UserAgent:     proto.String("curl/7.47.0"),
			StatusCode:    proto.Int32(200),
			ContentLength: proto.Int64(1234),
			ApplicationId: &events.UUID{
				High: proto.Uint64(5),
				Low:  proto.Uint64(8),
			},
			Forwarded:     []string{"serverA, serverB", "serverC"},
			InstanceIndex: proto.Int32(123),
			InstanceId:    proto.String("test-instance-id"),
		},
	}),
	Entry("LogMessage", &events.Envelope{
		Origin:     proto.String("some-origin"),
		Timestamp:  proto.Int64(1234),
		Deployment: proto.String("test-deployment"),
		Job:        proto.String("test-job"),
		Index:      proto.String("test-index"),
		Ip:         proto.String("test-ip"),
		Tags: map[string]string{
			"some-random": "tag",
			"source_id":   "some-app-id",
		},
		EventType: events.Envelope_LogMessage.Enum(),
		LogMessage: &events.LogMessage{
			Message:        []byte("some-message"),
			MessageType:    events.LogMessage_ERR.Enum(),
			Timestamp:      proto.Int64(1234),
			AppId:          proto.String("some-app-id"),
			SourceType:     proto.String("some-source-type"),
			SourceInstance: proto.String("some-source-instance"),
		},
	}),
	Entry("ValueMetric", &events.Envelope{
		Origin:     proto.String("some-origin"),
		Timestamp:  proto.Int64(1234),
		Deployment: proto.String("test-deployment"),
		Job:        proto.String("test-job"),
		Index:      proto.String("test-index"),
		Ip:         proto.String("test-ip"),
		Tags: map[string]string{
			"some-random": "tag",
			"source_id":   "test-deployment/test-job",
		},
		EventType: events.Envelope_ValueMetric.Enum(),
		ValueMetric: &events.ValueMetric{
			Name:  proto.String("some-name"),
			Value: proto.Float64(1.2345),
			Unit:  proto.String("some-unit"),
		},
	}),
	Entry("CounterEvent", &events.Envelope{
		Origin:     proto.String("some-origin"),
		Timestamp:  proto.Int64(1234),
		Deployment: proto.String("test-deployment"),
		Job:        proto.String("test-job"),
		Index:      proto.String("test-index"),
		Ip:         proto.String("test-ip"),
		Tags: map[string]string{
			"some-random": "tag",
			"source_id":   "test-deployment/test-job",
		},
		EventType: events.Envelope_CounterEvent.Enum(),
		CounterEvent: &events.CounterEvent{
			Name:  proto.String("some-name"),
			Delta: proto.Uint64(0),
			Total: proto.Uint64(4356782),
		},
	}),
	Entry("Error", &events.Envelope{
		Origin:     proto.String("some-origin"),
		Timestamp:  proto.Int64(1234),
		Deployment: proto.String("test-deployment"),
		Job:        proto.String("test-job"),
		Index:      proto.String("test-index"),
		Ip:         proto.String("test-ip"),
		Tags: map[string]string{
			"some-random": "tag",
			"source_id":   "test-deployment/test-job",
		},
		EventType: events.Envelope_Error.Enum(),
		Error: &events.Error{
			Source:  proto.String("some-source"),
			Code:    proto.Int32(12631),
			Message: proto.String("some-message"),
		},
	}),
	Entry("ContainerMetric", &events.Envelope{
		Origin:     proto.String("some-origin"),
		Timestamp:  proto.Int64(1234),
		Deployment: proto.String("test-deployment"),
		Job:        proto.String("test-job"),
		Index:      proto.String("test-index"),
		Ip:         proto.String("test-ip"),
		Tags: map[string]string{
			"some-random": "tag",
			"source_id":   "some-application-id",
		},
		EventType: events.Envelope_ContainerMetric.Enum(),
		ContainerMetric: &events.ContainerMetric{
			ApplicationId:    proto.String("some-application-id"),
			InstanceIndex:    proto.Int32(123546),
			CpuPercentage:    proto.Float64(1.12361),
			MemoryBytes:      proto.Uint64(213457),
			DiskBytes:        proto.Uint64(246583),
			MemoryBytesQuota: proto.Uint64(825456),
			DiskBytesQuota:   proto.Uint64(458724),
		},
	}),
)

var ValueText = func(s string) *v2.Value {
	return &v2.Value{&v2.Value_Text{Text: s}}
}

var ValueInteger = func(i int64) *v2.Value {
	return &v2.Value{&v2.Value_Integer{Integer: i}}
}

var _ = DescribeTable("v2->v1->v2",

	func(v2e *v2.Envelope) {
		_, err := proto.Marshal(v2e)
		Expect(err).ToNot(HaveOccurred())

		v1e := ToV1(v2e)

		_, err = proto.Marshal(v1e)
		Expect(err).ToNot(HaveOccurred())

		newV2e := ToV2(v1e)
		Expect(newV2e).To(Equal(v2e))
	},
	Entry("HttpStartStop", &v2.Envelope{
		Timestamp: time.Now().UnixNano(),
		SourceId:  "b3015d69-09cd-476d-aace-ad2d824d5ab7",
		Message: &v2.Envelope_Timer{
			Timer: &v2.Timer{
				Name:  "http",
				Start: 99,
				Stop:  100,
			},
		},
		Tags: map[string]*v2.Value{
			"request_id":     ValueText("954f61c4-ac84-44be-9217-cdfa3117fb41"),
			"peer_type":      ValueText("Client"),
			"method":         ValueText("GET"),
			"uri":            ValueText("/hello-world"),
			"remote_address": ValueText("10.1.1.0"),
			"user_agent":     ValueText("Mozilla/5.0"),
			"status_code":    ValueInteger(200),
			"content_length": ValueInteger(1000000),
			"instance_index": ValueInteger(10),
			"instance_id":    ValueText("application-id"),
			"forwarded":      ValueText("6.6.6.6\n8.8.8.8"),
			"deployment":     ValueText("some-deployment"),
			"ip":             ValueText("some-ip"),
			"job":            ValueText("some-job"),
			"origin":         ValueText("some-origin"),
			"index":          ValueText("some-index"),
			"__v1_type":      ValueText("HttpStartStop"),
		},
	}),
	Entry("Log", &v2.Envelope{
		Timestamp: time.Now().UnixNano(),
		SourceId:  "b3015d69-09cd-476d-aace-ad2d824d5ab7",
		Message: &v2.Envelope_Log{
			Log: &v2.Log{
				Payload: []byte("some-payload"),
				Type:    v2.Log_OUT,
			},
		},
		Tags: map[string]*v2.Value{
			"source_type":     ValueText("some-source-type"),
			"source_instance": ValueText("some-source-instance"),
			"deployment":      ValueText("some-deployment"),
			"ip":              ValueText("some-ip"),
			"job":             ValueText("some-job"),
			"origin":          ValueText("some-origin"),
			"index":           ValueText("some-index"),
			"__v1_type":       ValueText("LogMessage"),
		},
	}),
	Entry("Counter", &v2.Envelope{
		Timestamp: time.Now().UnixNano(),
		SourceId:  "b3015d69-09cd-476d-aace-ad2d824d5ab7",
		Message: &v2.Envelope_Counter{
			Counter: &v2.Counter{
				Name: "some-name",
				Value: &v2.Counter_Total{
					Total: 99,
				},
			},
		},
		Tags: map[string]*v2.Value{
			"deployment": ValueText("some-deployment"),
			"ip":         ValueText("some-ip"),
			"job":        ValueText("some-job"),
			"origin":     ValueText("some-origin"),
			"index":      ValueText("some-index"),
			"__v1_type":  ValueText("CounterEvent"),
		},
	}),
)
