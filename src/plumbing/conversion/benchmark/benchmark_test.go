package conversion_test

import (
	"testing"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator/plumbing/conversion"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

func BenchmarkToV2Log(b *testing.B) {
	for i := 0; i < b.N; i++ {
		outV2 = conversion.ToV2(&logMessageV1, true)
	}
}

func BenchmarkToV2HttpStartStop(b *testing.B) {
	for i := 0; i < b.N; i++ {
		outV2 = conversion.ToV2(&httpStartStopV1, true)
	}
}

func BenchmarkToV2ValueMetric(b *testing.B) {
	for i := 0; i < b.N; i++ {
		outV2 = conversion.ToV2(&valueMetricV1, true)
	}
}

func BenchmarkToV2CounterEvent(b *testing.B) {
	for i := 0; i < b.N; i++ {
		outV2 = conversion.ToV2(&counterEventV1, true)
	}
}

func BenchmarkToV1EnvelopeLog(b *testing.B) {
	for i := 0; i < b.N; i++ {
		outV1 = conversion.ToV1(envelopeLogV2)
	}
}

func BenchmarkToV1EnvelopeCounter(b *testing.B) {
	for i := 0; i < b.N; i++ {
		outV1 = conversion.ToV1(envelopeCounterV2)
	}
}

func BenchmarkToV1EnvelopeGauge(b *testing.B) {
	for i := 0; i < b.N; i++ {
		outV1 = conversion.ToV1(envelopeGaugeV2)
	}
}

func BenchmarkToV1EnvelopeTimer(b *testing.B) {
	for i := 0; i < b.N; i++ {
		outV1 = conversion.ToV1(envelopeTimerV2)
	}
}

var (
	outV1           []*events.Envelope
	outV2           *loggregator_v2.Envelope
	resultSlice     []byte
	err             error
	httpStartStopV1 = events.Envelope{
		Origin:     proto.String("origin"),
		Timestamp:  proto.Int64(time.Now().UnixNano()),
		EventType:  events.Envelope_HttpStartStop.Enum(),
		Deployment: proto.String("deployment"),
		Job:        proto.String("job-name"),
		Index:      proto.String("just-an-average-sized-index-id"),
		Ip:         proto.String("127.0.0.1"),
		HttpStartStop: &events.HttpStartStop{
			StartTimestamp: proto.Int64(time.Now().UnixNano()),
			StopTimestamp:  proto.Int64(time.Now().UnixNano()),
			RequestId: &events.UUID{
				Low:  proto.Uint64(12345),
				High: proto.Uint64(12345),
			},
			PeerType:      events.PeerType_Client.Enum(),
			Method:        events.Method_GET.Enum(),
			Uri:           proto.String("https://thing.thing.thing/thing"),
			RemoteAddress: proto.String("127.0.0.1"),
			UserAgent:     proto.String("super-realistic-agent"),
			StatusCode:    proto.Int32(123),
			ContentLength: proto.Int64(1234),
			ApplicationId: &events.UUID{
				Low:  proto.Uint64(12345),
				High: proto.Uint64(12345),
			},
			InstanceIndex: proto.Int32(2),
			InstanceId:    proto.String("a-really-medium-sized-guid-maybe"),
			Forwarded:     []string{"forwarded for someone"},
		},
		Tags: map[string]string{
			"adding":  "tags",
			"for":     "good",
			"measure": "mkay",
		},
	}
	logMessageV1 = events.Envelope{
		Origin:     proto.String("origin"),
		Timestamp:  proto.Int64(time.Now().UnixNano()),
		EventType:  events.Envelope_LogMessage.Enum(),
		Deployment: proto.String("deployment"),
		Job:        proto.String("job-name"),
		Index:      proto.String("just-an-average-sized-index-id"),
		Ip:         proto.String("127.0.0.1"),
		LogMessage: &events.LogMessage{
			Message:        []byte("a log message of a decent size that none would mine too much cause its not too big but not too small"),
			MessageType:    events.LogMessage_OUT.Enum(),
			Timestamp:      proto.Int64(time.Now().UnixNano()),
			AppId:          proto.String("just-an-average-sized-app-id"),
			SourceInstance: proto.String("just-an-average-sized-source-instance"),
			SourceType:     proto.String("something/medium"),
		},
		Tags: map[string]string{
			"adding":  "tags",
			"for":     "good",
			"measure": "mkay",
		},
	}
	valueMetricV1 = events.Envelope{
		Origin:     proto.String("origin"),
		Timestamp:  proto.Int64(time.Now().UnixNano()),
		EventType:  events.Envelope_ValueMetric.Enum(),
		Deployment: proto.String("deployment"),
		Job:        proto.String("job-name"),
		Index:      proto.String("just-an-average-sized-index-id"),
		Ip:         proto.String("127.0.0.1"),
		ValueMetric: &events.ValueMetric{
			Name:  proto.String("some-value-metric"),
			Value: proto.Float64(0.23232),
			Unit:  proto.String("things"),
		},
		Tags: map[string]string{
			"adding":  "tags",
			"for":     "good",
			"measure": "mkay",
		},
	}
	counterEventV1 = events.Envelope{
		Origin:     proto.String("origin"),
		Timestamp:  proto.Int64(time.Now().UnixNano()),
		EventType:  events.Envelope_CounterEvent.Enum(),
		Deployment: proto.String("deployment"),
		Job:        proto.String("job-name"),
		Index:      proto.String("just-an-average-sized-index-id"),
		Ip:         proto.String("127.0.0.1"),
		CounterEvent: &events.CounterEvent{
			Name:  proto.String("some-value-metric"),
			Total: proto.Uint64(23232),
		},
		Tags: map[string]string{
			"adding":  "tags",
			"for":     "good",
			"measure": "mkay",
		},
	}
	envelopeLogV2 = &loggregator_v2.Envelope{
		Timestamp:  time.Now().UnixNano(),
		SourceId:   "b3015d69-09cd-476d-aace-ad2d824d5ab7",
		InstanceId: "99",
		Message: &loggregator_v2.Envelope_Log{
			Log: &loggregator_v2.Log{
				Payload: []byte("some-payload"),
				Type:    loggregator_v2.Log_OUT,
			},
		},
		Tags: map[string]string{
			"source_type": "some-source-type",
			"deployment":  "some-deployment",
			"ip":          "some-ip",
			"job":         "some-job",
			"origin":      "some-origin",
			"index":       "some-index",
			"__v1_type":   "LogMessage",
		},
	}
	envelopeCounterV2 = &loggregator_v2.Envelope{
		Timestamp:  time.Now().UnixNano(),
		SourceId:   "b3015d69-09cd-476d-aace-ad2d824d5ab7",
		InstanceId: "99",
		Message: &loggregator_v2.Envelope_Counter{
			Counter: &loggregator_v2.Counter{
				Name:  "some-name",
				Total: 99,
			},
		},
		Tags: map[string]string{
			"deployment": "some-deployment",
			"ip":         "some-ip",
			"job":        "some-job",
			"origin":     "some-origin",
			"index":      "some-index",
			"__v1_type":  "CounterEvent",
		},
	}
	envelopeGaugeV2 = &loggregator_v2.Envelope{
		Timestamp:  time.Now().UnixNano(),
		SourceId:   "b3015d69-09cd-476d-aace-ad2d824d5ab7",
		InstanceId: "99",
		Message: &loggregator_v2.Envelope_Gauge{
			Gauge: &loggregator_v2.Gauge{
				Metrics: map[string]*loggregator_v2.GaugeValue{
					"cpu": {
						Unit: "percentage", Value: 0.18079146710267877,
					},
					"disk": {
						Unit: "bytes", Value: 7.9466496e+07,
					},
					"disk_quota": {
						Unit: "bytes", Value: 1.073741824e+09,
					},
					"memory": {
						Unit: "bytes", Value: 2.5223168e+07,
					},
					"memory_quota": {
						Unit: "bytes", Value: 2.68435456e+08,
					},
				},
			},
		},
		Tags: map[string]string{
			"deployment": "some-deployment",
			"ip":         "some-ip",
			"job":        "some-job",
			"origin":     "some-origin",
			"index":      "some-index",
			"__v1_type":  "ContainerMetric",
		},
	}
	envelopeTimerV2 = &loggregator_v2.Envelope{
		Timestamp:  time.Now().UnixNano(),
		SourceId:   "b3015d69-09cd-476d-aace-ad2d824d5ab7",
		InstanceId: "99",
		Message: &loggregator_v2.Envelope_Timer{
			Timer: &loggregator_v2.Timer{
				Name:  "http",
				Start: 99,
				Stop:  100,
			},
		},
		Tags: map[string]string{
			"request_id":          "954f61c4-ac84-44be-9217-cdfa3117fb41",
			"peer_type":           "Client",
			"method":              "GET",
			"uri":                 "/hello-world",
			"remote_address":      "10.1.1.0",
			"user_agent":          "Mozilla/5.0",
			"status_code":         "200",
			"content_length":      "1000000",
			"routing_instance_id": "application-id",
			"forwarded":           "6.6.6.6\n8.8.8.8",
			"deployment":          "some-deployment",
			"ip":                  "some-ip",
			"job":                 "some-job",
			"origin":              "some-origin",
			"index":               "some-index",
			"__v1_type":           "HttpStartStop",
		},
	}
)
