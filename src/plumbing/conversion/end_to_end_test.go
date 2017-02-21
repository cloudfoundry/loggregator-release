package conversion_test

import (
	. "plumbing/conversion"

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
