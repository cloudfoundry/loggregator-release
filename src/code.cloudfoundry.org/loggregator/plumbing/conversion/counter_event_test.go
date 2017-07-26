package conversion_test

import (
	"code.cloudfoundry.org/loggregator/plumbing/conversion"
	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("CounterEvent", func() {
	Context("given a v2 envelope", func() {
		It("converts to a v1 envelope", func() {
			envelope := &v2.Envelope{
				Message: &v2.Envelope_Counter{
					Counter: &v2.Counter{
						Name: "name",
						Value: &v2.Counter_Total{
							Total: 99,
						},
					},
				},
			}

			envelopes := conversion.ToV1(envelope)
			Expect(len(envelopes)).To(Equal(1))
			Expect(*envelopes[0]).To(MatchFields(IgnoreExtras, Fields{
				"EventType": Equal(events.Envelope_CounterEvent.Enum()),
				"CounterEvent": Equal(&events.CounterEvent{
					Name:  proto.String("name"),
					Total: proto.Uint64(99),
					Delta: proto.Uint64(0),
				}),
			}))
		})
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
				EventType:  events.Envelope_CounterEvent.Enum(),
				CounterEvent: &events.CounterEvent{
					Name:  proto.String("name"),
					Total: proto.Uint64(99),
				},
				Tags: map[string]string{
					"custom_tag":  "custom-value",
					"source_id":   "source-id",
					"instance_id": "instance-id",
				},
			}
			v2Envelope = &v2.Envelope{
				Message: &v2.Envelope_Counter{
					Counter: &v2.Counter{
						Name: "name",
						Value: &v2.Counter_Total{
							Total: 99,
						},
					},
				},
			}
		})

		Context("using deprecated tags", func() {
			It("converts to a v2 envelope with DeprecatedTags", func() {
				Expect(*conversion.ToV2(v1Envelope, false)).To(MatchFields(0, Fields{
					"Timestamp":  Equal(int64(1234)),
					"SourceId":   Equal("source-id"),
					"InstanceId": Equal("instance-id"),
					"Message":    Equal(v2Envelope.Message),
					"DeprecatedTags": Equal(map[string]*v2.Value{
						"origin":     {Data: &v2.Value_Text{Text: "an-origin"}},
						"deployment": {Data: &v2.Value_Text{Text: "a-deployment"}},
						"job":        {Data: &v2.Value_Text{Text: "a-job"}},
						"index":      {Data: &v2.Value_Text{Text: "an-index"}},
						"ip":         {Data: &v2.Value_Text{Text: "an-ip"}},
						"__v1_type":  {Data: &v2.Value_Text{Text: "CounterEvent"}},
						"custom_tag": {Data: &v2.Value_Text{Text: "custom-value"}},
					}),
					"Tags": BeNil(),
				}))
			})
		})

		Context("using preferred tags", func() {
			It("converts to a v2 envelope with Tags", func() {
				Expect(*conversion.ToV2(v1Envelope, true)).To(MatchFields(0, Fields{
					"Timestamp":      Equal(int64(1234)),
					"SourceId":       Equal("source-id"),
					"InstanceId":     Equal("instance-id"),
					"Message":        Equal(v2Envelope.Message),
					"DeprecatedTags": BeNil(),
					"Tags": Equal(map[string]string{
						"origin":     "an-origin",
						"deployment": "a-deployment",
						"job":        "a-job",
						"index":      "an-index",
						"ip":         "an-ip",
						"__v1_type":  "CounterEvent",
						"custom_tag": "custom-value",
					}),
				}))
			})
		})
	})
})
