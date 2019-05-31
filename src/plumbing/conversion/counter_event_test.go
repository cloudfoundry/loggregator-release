package conversion_test

import (
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator/plumbing/conversion"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("CounterEvent", func() {
	Context("given a v2 envelope", func() {
		Context("with a total", func() {
			It("converts to a v1 envelope", func() {
				envelope := &loggregator_v2.Envelope{
					Message: &loggregator_v2.Envelope_Counter{
						Counter: &loggregator_v2.Counter{
							Name:  "name",
							Total: 99,
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

		Context("with a delta", func() {
			It("converts to a v1 envelope", func() {
				envelope := &loggregator_v2.Envelope{
					Message: &loggregator_v2.Envelope_Counter{
						Counter: &loggregator_v2.Counter{
							Name:  "name",
							Delta: 99,
						},
					},
				}

				envelopes := conversion.ToV1(envelope)
				Expect(len(envelopes)).To(Equal(1))
				Expect(*envelopes[0]).To(MatchFields(IgnoreExtras, Fields{
					"EventType": Equal(events.Envelope_CounterEvent.Enum()),
					"CounterEvent": Equal(&events.CounterEvent{
						Name:  proto.String("name"),
						Total: proto.Uint64(0),
						Delta: proto.Uint64(99),
					}),
				}))
			})
		})
	})

	Context("given a v1 envelope", func() {
		var (
			v1Envelope      *events.Envelope
			expectedMessage *loggregator_v2.Envelope_Counter
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
					Delta: proto.Uint64(2),
				},
				Tags: map[string]string{
					"custom_tag":  "custom-value",
					"source_id":   "source-id",
					"instance_id": "instance-id",
				},
			}
			expectedMessage = &loggregator_v2.Envelope_Counter{
				Counter: &loggregator_v2.Counter{
					Name:  "name",
					Delta: 2,
					Total: 99,
				},
			}
		})

		Context("using deprecated tags", func() {
			It("converts to a v2 envelope with DeprecatedTags", func() {
				Expect(*conversion.ToV2(v1Envelope, false)).To(MatchFields(IgnoreExtras, Fields{
					"Timestamp":  Equal(int64(1234)),
					"SourceId":   Equal("source-id"),
					"InstanceId": Equal("instance-id"),
					"Message":    Equal(expectedMessage),
					"DeprecatedTags": Equal(map[string]*loggregator_v2.Value{
						"origin":     {Data: &loggregator_v2.Value_Text{Text: "an-origin"}},
						"deployment": {Data: &loggregator_v2.Value_Text{Text: "a-deployment"}},
						"job":        {Data: &loggregator_v2.Value_Text{Text: "a-job"}},
						"index":      {Data: &loggregator_v2.Value_Text{Text: "an-index"}},
						"ip":         {Data: &loggregator_v2.Value_Text{Text: "an-ip"}},
						"__v1_type":  {Data: &loggregator_v2.Value_Text{Text: "CounterEvent"}},
						"custom_tag": {Data: &loggregator_v2.Value_Text{Text: "custom-value"}},
					}),
					"Tags": BeNil(),
				}))
			})
		})

		Context("using preferred tags", func() {
			It("converts to a v2 envelope with Tags", func() {
				Expect(*conversion.ToV2(v1Envelope, true)).To(MatchFields(IgnoreExtras, Fields{
					"Timestamp":      Equal(int64(1234)),
					"SourceId":       Equal("source-id"),
					"InstanceId":     Equal("instance-id"),
					"Message":        Equal(expectedMessage),
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
