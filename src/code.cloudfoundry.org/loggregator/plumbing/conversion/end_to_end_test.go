package conversion_test

import (
	"time"

	. "code.cloudfoundry.org/loggregator/plumbing/conversion"
	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Envelope conversion", func() {
	Context("v1->v2->v1", func() {
		It("converts HttpStartStop", func() {
			v1e := &events.Envelope{
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
			}

			_, err := proto.Marshal(v1e)
			Expect(err).ToNot(HaveOccurred())

			v2e := ToV2(v1e, false)

			_, err = proto.Marshal(v2e)
			Expect(err).ToNot(HaveOccurred())

			v1Envs := ToV1(v2e)
			Expect(len(v1Envs)).To(Equal(1))

			newV1e := v1Envs[0]
			Expect(newV1e).To(Equal(v1e))
		})

		It("converts LogMessage", func() {
			v1e := &events.Envelope{
				Origin:     proto.String("some-origin"),
				Timestamp:  proto.Int64(1234),
				Deployment: proto.String("test-deployment"),
				Job:        proto.String("test-job"),
				Index:      proto.String("test-index"),
				Ip:         proto.String("test-ip"),
				Tags: map[string]string{
					"some-random": "tag",
					"source_id":   "some-app-id",
					"instance_id": "some-source-instance",
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
			}

			_, err := proto.Marshal(v1e)
			Expect(err).ToNot(HaveOccurred())

			v2e := ToV2(v1e, false)

			_, err = proto.Marshal(v2e)
			Expect(err).ToNot(HaveOccurred())

			v1Envs := ToV1(v2e)
			Expect(len(v1Envs)).To(Equal(1))

			newV1e := v1Envs[0]
			Expect(newV1e).To(Equal(v1e))
		})

		It("converts ValueMetric", func() {
			v1e := &events.Envelope{
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
			}

			_, err := proto.Marshal(v1e)
			Expect(err).ToNot(HaveOccurred())

			v2e := ToV2(v1e, false)

			_, err = proto.Marshal(v2e)
			Expect(err).ToNot(HaveOccurred())

			v1Envs := ToV1(v2e)
			Expect(len(v1Envs)).To(Equal(1))

			newV1e := v1Envs[0]
			Expect(newV1e).To(Equal(v1e))
		})

		It("converts CounterEvent", func() {
			v1e := &events.Envelope{
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
			}

			_, err := proto.Marshal(v1e)
			Expect(err).ToNot(HaveOccurred())

			v2e := ToV2(v1e, false)

			_, err = proto.Marshal(v2e)
			Expect(err).ToNot(HaveOccurred())

			v1Envs := ToV1(v2e)
			Expect(len(v1Envs)).To(Equal(1))

			newV1e := v1Envs[0]
			Expect(newV1e).To(Equal(v1e))
		})

		It("converts Error", func() {
			v1e := &events.Envelope{
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
			}

			_, err := proto.Marshal(v1e)
			Expect(err).ToNot(HaveOccurred())

			v2e := ToV2(v1e, false)

			_, err = proto.Marshal(v2e)
			Expect(err).ToNot(HaveOccurred())

			v1Envs := ToV1(v2e)
			Expect(len(v1Envs)).To(Equal(1))

			newV1e := v1Envs[0]
			Expect(newV1e).To(Equal(v1e))
		})

		It("ContainerMetric", func() {
			v1e := &events.Envelope{
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
			}

			_, err := proto.Marshal(v1e)
			Expect(err).ToNot(HaveOccurred())

			v2e := ToV2(v1e, false)

			_, err = proto.Marshal(v2e)
			Expect(err).ToNot(HaveOccurred())

			v1Envs := ToV1(v2e)
			Expect(len(v1Envs)).To(Equal(1))

			newV1e := v1Envs[0]
			Expect(newV1e).To(Equal(v1e))
		})
	})

	Context("v2->v1->v2", func() {
		It("converts HttpStartStop", func() {
			v2e := &v2.Envelope{
				Timestamp:  time.Now().UnixNano(),
				SourceId:   "b3015d69-09cd-476d-aace-ad2d824d5ab7",
				InstanceId: "99",
				Message: &v2.Envelope_Timer{
					Timer: &v2.Timer{
						Name:  "http",
						Start: 99,
						Stop:  100,
					},
				},
				DeprecatedTags: map[string]*v2.Value{
					"request_id":          ValueText("954f61c4-ac84-44be-9217-cdfa3117fb41"),
					"peer_type":           ValueText("Client"),
					"method":              ValueText("GET"),
					"uri":                 ValueText("/hello-world"),
					"remote_address":      ValueText("10.1.1.0"),
					"user_agent":          ValueText("Mozilla/5.0"),
					"status_code":         ValueInteger(200),
					"content_length":      ValueInteger(1000000),
					"instance_index":      ValueInteger(10),
					"routing_instance_id": ValueText("application-id"),
					"forwarded":           ValueText("6.6.6.6\n8.8.8.8"),
					"deployment":          ValueText("some-deployment"),
					"ip":                  ValueText("some-ip"),
					"job":                 ValueText("some-job"),
					"origin":              ValueText("some-origin"),
					"index":               ValueText("some-index"),
					"__v1_type":           ValueText("HttpStartStop"),
				},
			}

			_, err := proto.Marshal(v2e)
			Expect(err).ToNot(HaveOccurred())

			envelopes := ToV1(v2e)
			Expect(len(envelopes)).To(Equal(1))
			v1e := envelopes[0]

			_, err = proto.Marshal(v1e)
			Expect(err).ToNot(HaveOccurred())

			v1Envs := ToV1(v2e)
			Expect(len(v1Envs)).To(Equal(1))

			newV1e := v1Envs[0]
			Expect(newV1e).To(Equal(v1e))
		})

		It("converts Log", func() {
			v2e := &v2.Envelope{
				Timestamp:  time.Now().UnixNano(),
				SourceId:   "b3015d69-09cd-476d-aace-ad2d824d5ab7",
				InstanceId: "99",
				Message: &v2.Envelope_Log{
					Log: &v2.Log{
						Payload: []byte("some-payload"),
						Type:    v2.Log_OUT,
					},
				},
				DeprecatedTags: map[string]*v2.Value{
					"source_type": ValueText("some-source-type"),
					"deployment":  ValueText("some-deployment"),
					"ip":          ValueText("some-ip"),
					"job":         ValueText("some-job"),
					"origin":      ValueText("some-origin"),
					"index":       ValueText("some-index"),
					"__v1_type":   ValueText("LogMessage"),
				},
			}

			_, err := proto.Marshal(v2e)
			Expect(err).ToNot(HaveOccurred())

			envelopes := ToV1(v2e)
			Expect(len(envelopes)).To(Equal(1))
			v1e := envelopes[0]

			_, err = proto.Marshal(v1e)
			Expect(err).ToNot(HaveOccurred())

			v1Envs := ToV1(v2e)
			Expect(len(v1Envs)).To(Equal(1))

			newV1e := v1Envs[0]
			Expect(newV1e).To(Equal(v1e))
		})

		It("converts Counter", func() {
			v2e := &v2.Envelope{
				Timestamp:  time.Now().UnixNano(),
				SourceId:   "b3015d69-09cd-476d-aace-ad2d824d5ab7",
				InstanceId: "99",
				Message: &v2.Envelope_Counter{
					Counter: &v2.Counter{
						Name: "some-name",
						Value: &v2.Counter_Total{
							Total: 99,
						},
					},
				},
				DeprecatedTags: map[string]*v2.Value{
					"deployment": ValueText("some-deployment"),
					"ip":         ValueText("some-ip"),
					"job":        ValueText("some-job"),
					"origin":     ValueText("some-origin"),
					"index":      ValueText("some-index"),
					"__v1_type":  ValueText("CounterEvent"),
				},
			}

			_, err := proto.Marshal(v2e)
			Expect(err).ToNot(HaveOccurred())

			envelopes := ToV1(v2e)
			Expect(len(envelopes)).To(Equal(1))
			v1e := envelopes[0]

			_, err = proto.Marshal(v1e)
			Expect(err).ToNot(HaveOccurred())

			v1Envs := ToV1(v2e)
			Expect(len(v1Envs)).To(Equal(1))

			newV1e := v1Envs[0]
			Expect(newV1e).To(Equal(v1e))
		})

		It("converts Gauge", func() {
			v2e := &v2.Envelope{
				Timestamp:  time.Now().UnixNano(),
				SourceId:   "b3015d69-09cd-476d-aace-ad2d824d5ab7",
				InstanceId: "99",
				Message: &v2.Envelope_Gauge{
					Gauge: &v2.Gauge{
						Metrics: map[string]*v2.GaugeValue{
							"cpu": {
								Unit: "percentage", Value: 0.18079146710267877,
							},
							"disk": {
								Unit: "bytes", Value: 7.9466496e+07,
							},
							"disk_quota": {
								Unit: "bytes", Value: 1.073741824e+09,
							},
							"instance_index": {
								Unit: "index", Value: 0,
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
				DeprecatedTags: map[string]*v2.Value{
					"deployment": ValueText("some-deployment"),
					"ip":         ValueText("some-ip"),
					"job":        ValueText("some-job"),
					"origin":     ValueText("some-origin"),
					"index":      ValueText("some-index"),
					"__v1_type":  ValueText("ContainerMetric"),
				},
			}

			_, err := proto.Marshal(v2e)
			Expect(err).ToNot(HaveOccurred())

			envelopes := ToV1(v2e)
			Expect(len(envelopes)).To(Equal(1))
			v1e := envelopes[0]

			_, err = proto.Marshal(v1e)
			Expect(err).ToNot(HaveOccurred())

			v1Envs := ToV1(v2e)
			Expect(len(v1Envs)).To(Equal(1))

			newV1e := v1Envs[0]
			Expect(newV1e).To(Equal(v1e))
		})
	})
})

func ValueText(s string) *v2.Value {
	return &v2.Value{&v2.Value_Text{Text: s}}
}

func ValueInteger(i int64) *v2.Value {
	return &v2.Value{&v2.Value_Integer{Integer: i}}
}
