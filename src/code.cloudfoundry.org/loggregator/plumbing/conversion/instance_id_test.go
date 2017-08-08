package conversion_test

import (
	"code.cloudfoundry.org/loggregator/plumbing/conversion"
	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Converting Instance IDs", func() {
	Describe("LogMessage", func() {
		It("reads the v1 source_instance field when converting to v2", func() {
			v1Envelope := &events.Envelope{
				Origin:    proto.String(""),
				EventType: events.Envelope_LogMessage.Enum(),
				LogMessage: &events.LogMessage{
					Message:        []byte(""),
					MessageType:    events.LogMessage_OUT.Enum(),
					Timestamp:      proto.Int64(0),
					SourceInstance: proto.String("test-source-instance"),
				},
			}
			actualV2Envelope := conversion.ToV2(v1Envelope, false)
			Expect(actualV2Envelope.InstanceId).To(Equal("test-source-instance"))
		})

		It("writes into the v1 source_instance field when converting to v1", func() {
			v2Envelope := &v2.Envelope{
				InstanceId: "test-source-instance",
				Message: &v2.Envelope_Log{
					Log: &v2.Log{
						Payload: []byte("Hello World"),
						Type:    v2.Log_OUT,
					},
				},
			}
			envelopes := conversion.ToV1(v2Envelope)
			Expect(len(envelopes)).To(Equal(1))
			Expect(*envelopes[0].LogMessage.SourceInstance).To(Equal("test-source-instance"))
		})
	})

	Describe("HttpStartStop", func() {
		It("reads the v1 instance_index field when converting to v2", func() {
			v1Envelope := &events.Envelope{
				Origin:    proto.String(""),
				EventType: events.Envelope_HttpStartStop.Enum(),
				HttpStartStop: &events.HttpStartStop{
					StartTimestamp: proto.Int64(0),
					StopTimestamp:  proto.Int64(0),
					RequestId:      &events.UUID{},
					PeerType:       events.PeerType_Client.Enum(),
					Method:         events.Method_GET.Enum(),
					Uri:            proto.String(""),
					RemoteAddress:  proto.String(""),
					UserAgent:      proto.String(""),
					StatusCode:     proto.Int32(0),
					ContentLength:  proto.Int64(0),
					InstanceIndex:  proto.Int32(1234),
				},
			}
			actualV2Envelope := conversion.ToV2(v1Envelope, false)
			Expect(actualV2Envelope.InstanceId).To(Equal("1234"))
		})

		It("writes into the v1 instance_index field when converting to v1", func() {
			v2Envelope := &v2.Envelope{
				InstanceId: "1234",
				Message: &v2.Envelope_Timer{
					Timer: &v2.Timer{},
				},
			}
			envelopes := conversion.ToV1(v2Envelope)
			Expect(len(envelopes)).To(Equal(1))
			Expect(*envelopes[0].HttpStartStop.InstanceIndex).To(Equal(int32(1234)))
		})

		It("writes 0 into the v1 instance_index field if instance_id is not an int", func() {
			v2Envelope := &v2.Envelope{
				InstanceId: "garbage",
				Message: &v2.Envelope_Timer{
					Timer: &v2.Timer{},
				},
			}
			envelopes := conversion.ToV1(v2Envelope)
			Expect(len(envelopes)).To(Equal(1))
			Expect(*envelopes[0].HttpStartStop.InstanceIndex).To(Equal(int32(0)))
		})
	})

	Describe("ContainerMetric", func() {
		It("reads the v1 instance_index field when converting to v2", func() {
			v1Envelope := &events.Envelope{
				Origin:    proto.String(""),
				EventType: events.Envelope_ContainerMetric.Enum(),
				ContainerMetric: &events.ContainerMetric{
					ApplicationId: proto.String(""),
					InstanceIndex: proto.Int32(4321),
					CpuPercentage: proto.Float64(0),
					MemoryBytes:   proto.Uint64(0),
					DiskBytes:     proto.Uint64(0),
				},
			}
			actualV2Envelope := conversion.ToV2(v1Envelope, false)
			Expect(actualV2Envelope.InstanceId).To(Equal("4321"))
		})

		It("writes into the v1 instance_index field when converting to v1", func() {
			v2Envelope := &v2.Envelope{
				InstanceId: "4321",
				Message: &v2.Envelope_Gauge{
					Gauge: &v2.Gauge{
						Metrics: map[string]*v2.GaugeValue{
							"cpu":          {},
							"memory":       {},
							"disk":         {},
							"memory_quota": {},
							"disk_quota":   {},
						},
					},
				},
			}
			envelopes := conversion.ToV1(v2Envelope)
			Expect(len(envelopes)).To(Equal(1))
			Expect(*envelopes[0].ContainerMetric.InstanceIndex).To(Equal(int32(4321)))
		})

		It("writes 0 into the v1 instance_index field if instance_id is not an int", func() {
			v2Envelope := &v2.Envelope{
				InstanceId: "garbage",
				Message: &v2.Envelope_Gauge{
					Gauge: &v2.Gauge{
						Metrics: map[string]*v2.GaugeValue{
							"cpu":          {},
							"memory":       {},
							"disk":         {},
							"memory_quota": {},
							"disk_quota":   {},
						},
					},
				},
			}
			envelopes := conversion.ToV1(v2Envelope)
			Expect(len(envelopes)).To(Equal(1))
			Expect(*envelopes[0].ContainerMetric.InstanceIndex).To(Equal(int32(0)))
		})
	})

	Describe("CounterEvent and ValueMetric", func() {
		DescribeTable("reads the v1 instance_id tag when converting to v2", func(v1Envelope *events.Envelope) {
			actualV2Envelope := conversion.ToV2(v1Envelope, false)
			Expect(actualV2Envelope.InstanceId).To(Equal("test-source-instance"))
		},
			Entry("CounterEvent", &events.Envelope{
				Origin:    proto.String(""),
				EventType: events.Envelope_CounterEvent.Enum(),
				CounterEvent: &events.CounterEvent{
					Name:  proto.String(""),
					Delta: proto.Uint64(0),
				},
				Tags: map[string]string{
					"instance_id": "test-source-instance",
				},
			}),
			Entry("ValueMetric", &events.Envelope{
				Origin:    proto.String(""),
				EventType: events.Envelope_ValueMetric.Enum(),
				ValueMetric: &events.ValueMetric{
					Name:  proto.String(""),
					Value: proto.Float64(0),
					Unit:  proto.String(""),
				},
				Tags: map[string]string{
					"instance_id": "test-source-instance",
				},
			}),
		)

		DescribeTable("writes into the v1 instance_id tag when converting to v1", func(v2Envelope *v2.Envelope) {
			envelopes := conversion.ToV1(v2Envelope)
			Expect(len(envelopes)).To(Equal(1))
			Expect(envelopes[0].Tags["instance_id"]).To(Equal("test-source-instance"))
		},
			Entry("CounterEvent", &v2.Envelope{
				InstanceId: "test-source-instance",
				Message: &v2.Envelope_Counter{
					Counter: &v2.Counter{},
				},
			}),
			Entry("ValueMetric", &v2.Envelope{
				InstanceId: "test-source-instance",
				Message: &v2.Envelope_Gauge{
					Gauge: &v2.Gauge{
						Metrics: map[string]*v2.GaugeValue{
							"some-metric": {
								Unit:  "test",
								Value: 123.4,
							},
						},
					},
				},
			}),
		)
	})
})
