package conversion_test

import (
	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"

	"code.cloudfoundry.org/loggregator/plumbing/conversion"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("ContainerMetric", func() {
	Context("given a v2 envelope", func() {
		It("converts to a v1 envelope", func() {
			envelope := &v2.Envelope{
				SourceId: "some-id",
				Message: &v2.Envelope_Gauge{
					Gauge: &v2.Gauge{
						Metrics: map[string]*v2.GaugeValue{
							"instance_index": {
								Unit:  "index",
								Value: 123,
							},
							"cpu": {
								Unit:  "percentage",
								Value: 11,
							},
							"memory": {
								Unit:  "bytes",
								Value: 13,
							},
							"disk": {
								Unit:  "bytes",
								Value: 15,
							},
							"memory_quota": {
								Unit:  "bytes",
								Value: 17,
							},
							"disk_quota": {
								Unit:  "bytes",
								Value: 19,
							},
						},
					},
				},
			}

			envelopes := conversion.ToV1(envelope)
			Expect(len(envelopes)).To(Equal(1))
			Expect(*envelopes[0]).To(MatchFields(IgnoreExtras, Fields{
				"EventType": Equal(events.Envelope_ContainerMetric.Enum()),
				"ContainerMetric": Equal(&events.ContainerMetric{
					ApplicationId:    proto.String("some-id"),
					InstanceIndex:    proto.Int32(123),
					CpuPercentage:    proto.Float64(11),
					MemoryBytes:      proto.Uint64(13),
					DiskBytes:        proto.Uint64(15),
					MemoryBytesQuota: proto.Uint64(17),
					DiskBytesQuota:   proto.Uint64(19),
				}),
			}))
		})

		DescribeTable("it is resilient to malformed envelopes", func(v2e *v2.Envelope) {
			Expect(conversion.ToV1(v2e)).To(BeNil())
		},
			Entry("bare envelope", &v2.Envelope{}),
			Entry("with empty fields", &v2.Envelope{
				Message: &v2.Envelope_Gauge{
					Gauge: &v2.Gauge{
						Metrics: map[string]*v2.GaugeValue{
							"instance_index": nil,
							"cpu":            nil,
							"memory":         nil,
							"disk":           nil,
							"memory_quota":   nil,
							"disk_quota":     nil,
						},
					},
				},
			}),
		)
	})

	Context("given a v1 envelope", func() {
		var (
			v1Envelope *events.Envelope
			v2Envelope *v2.Envelope
		)

		BeforeEach(func() {
			v1Envelope = &events.Envelope{
				Origin:     proto.String("an-origin"),
				Deployment: proto.String("a-deployment"),
				Job:        proto.String("a-job"),
				Index:      proto.String("an-index"),
				Ip:         proto.String("an-ip"),
				Timestamp:  proto.Int64(1234),
				EventType:  events.Envelope_ContainerMetric.Enum(),
				ContainerMetric: &events.ContainerMetric{
					ApplicationId:    proto.String("some-id"),
					InstanceIndex:    proto.Int32(123),
					CpuPercentage:    proto.Float64(11),
					MemoryBytes:      proto.Uint64(13),
					DiskBytes:        proto.Uint64(15),
					MemoryBytesQuota: proto.Uint64(17),
					DiskBytesQuota:   proto.Uint64(19),
				},
				Tags: map[string]string{
					"custom_tag":  "custom-value",
					"instance_id": "instance-id",
				},
			}
			v2Envelope = &v2.Envelope{
				SourceId: "some-id",
				Message: &v2.Envelope_Gauge{
					Gauge: &v2.Gauge{
						Metrics: map[string]*v2.GaugeValue{
							"instance_index": {
								Unit:  "index",
								Value: 123,
							},
							"cpu": {
								Unit:  "percentage",
								Value: 11,
							},
							"memory": {
								Unit:  "bytes",
								Value: 13,
							},
							"disk": {
								Unit:  "bytes",
								Value: 15,
							},
							"memory_quota": {
								Unit:  "bytes",
								Value: 17,
							},
							"disk_quota": {
								Unit:  "bytes",
								Value: 19,
							},
						},
					},
				},
			}
		})

		Context("using deprecated tags", func() {
			It("converts to a v2 envelope using DeprecatedTags", func() {
				Expect(*conversion.ToV2(v1Envelope, false)).To(MatchFields(0, Fields{
					"Timestamp":  Equal(int64(1234)),
					"SourceId":   Equal(v2Envelope.SourceId),
					"Message":    Equal(v2Envelope.Message),
					"InstanceId": Equal("instance-id"),
					"DeprecatedTags": Equal(map[string]*v2.Value{
						"origin":     {Data: &v2.Value_Text{Text: "an-origin"}},
						"deployment": {Data: &v2.Value_Text{Text: "a-deployment"}},
						"job":        {Data: &v2.Value_Text{Text: "a-job"}},
						"index":      {Data: &v2.Value_Text{Text: "an-index"}},
						"ip":         {Data: &v2.Value_Text{Text: "an-ip"}},
						"__v1_type":  {Data: &v2.Value_Text{Text: "ContainerMetric"}},
						"custom_tag": {Data: &v2.Value_Text{Text: "custom-value"}},
					}),
					"Tags": BeNil(),
				}))
			})

			It("sets the source ID to deployment/job when App ID is missing", func() {
				v1Envelope := &events.Envelope{
					Deployment:      proto.String("some-deployment"),
					Job:             proto.String("some-job"),
					EventType:       events.Envelope_ContainerMetric.Enum(),
					ContainerMetric: &events.ContainerMetric{},
				}

				expectedV2Envelope := &v2.Envelope{
					SourceId: "some-deployment/some-job",
				}

				converted := conversion.ToV2(v1Envelope, false)

				Expect(*converted).To(MatchFields(IgnoreExtras, Fields{
					"SourceId": Equal(expectedV2Envelope.SourceId),
				}))
			})
		})

		Context("using preferred tags", func() {
			It("converts to a v2 envelope using Tags", func() {
				Expect(*conversion.ToV2(v1Envelope, true)).To(MatchFields(0, Fields{
					"Timestamp":      Equal(int64(1234)),
					"SourceId":       Equal(v2Envelope.SourceId),
					"Message":        Equal(v2Envelope.Message),
					"InstanceId":     Equal("instance-id"),
					"DeprecatedTags": BeNil(),
					"Tags": Equal(map[string]string{
						"origin":     "an-origin",
						"deployment": "a-deployment",
						"job":        "a-job",
						"index":      "an-index",
						"ip":         "an-ip",
						"__v1_type":  "ContainerMetric",
						"custom_tag": "custom-value",
					}),
				}))
			})
		})
	})
})
