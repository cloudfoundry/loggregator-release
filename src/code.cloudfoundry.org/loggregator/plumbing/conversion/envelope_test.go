package conversion_test

import (
	"fmt"

	"code.cloudfoundry.org/loggregator/plumbing/conversion"
	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Envelope", func() {
	ValueText := func(s string) *v2.Value {
		return &v2.Value{&v2.Value_Text{Text: s}}
	}

	Context("given a v2 envelope", func() {
		It("sets v1 specific properties", func() {
			envelope := &v2.Envelope{
				Timestamp: 99,
				DeprecatedTags: map[string]*v2.Value{
					"origin":         {&v2.Value_Text{"origin"}},
					"deployment":     {&v2.Value_Text{"deployment"}},
					"job":            {&v2.Value_Text{"job"}},
					"index":          {&v2.Value_Text{"index"}},
					"ip":             {&v2.Value_Text{"ip"}},
					"random_text":    {&v2.Value_Text{"random_text"}},
					"random_int":     {&v2.Value_Integer{123}},
					"random_decimal": {&v2.Value_Decimal{123}},
				},
				Message: &v2.Envelope_Log{Log: &v2.Log{}},
			}

			envelopes := conversion.ToV1(envelope)
			Expect(len(envelopes)).To(Equal(1))
			oldEnvelope := envelopes[0]
			Expect(*oldEnvelope).To(MatchFields(IgnoreExtras, Fields{
				"Origin":     Equal(proto.String("origin")),
				"EventType":  Equal(events.Envelope_LogMessage.Enum()),
				"Timestamp":  Equal(proto.Int64(99)),
				"Deployment": Equal(proto.String("deployment")),
				"Job":        Equal(proto.String("job")),
				"Index":      Equal(proto.String("index")),
				"Ip":         Equal(proto.String("ip")),
			}))
			Expect(oldEnvelope.Tags).To(HaveKeyWithValue("random_text", "random_text"))
			Expect(oldEnvelope.Tags).To(HaveKeyWithValue("random_int", "123"))
			Expect(oldEnvelope.Tags).To(HaveKeyWithValue("random_decimal", fmt.Sprintf("%f", 123.0)))
		})

		It("rejects empty tags", func() {
			envelope := &v2.Envelope{
				DeprecatedTags: map[string]*v2.Value{
					"foo": {&v2.Value_Text{"bar"}},
					"baz": nil,
				},
				Message: &v2.Envelope_Log{Log: &v2.Log{}},
			}

			envelopes := conversion.ToV1(envelope)
			Expect(len(envelopes)).To(Equal(1))
			oldEnvelope := envelopes[0]
			Expect(oldEnvelope.Tags).To(Equal(map[string]string{
				"foo": "bar",
			}))
		})

		It("reads non-text v2 tags", func() {
			envelope := &v2.Envelope{
				DeprecatedTags: map[string]*v2.Value{
					"foo": {&v2.Value_Integer{99}},
				},
				Message: &v2.Envelope_Log{Log: &v2.Log{}},
			}

			envelopes := conversion.ToV1(envelope)
			Expect(len(envelopes)).To(Equal(1))
			Expect(envelopes[0].GetTags()).To(HaveKeyWithValue("foo", "99"))
		})

		It("uses non-deprecated v2 tags", func() {
			envelope := &v2.Envelope{
				Tags: map[string]string{
					"foo": "bar",
				},
				Message: &v2.Envelope_Log{Log: &v2.Log{}},
			}

			envelopes := conversion.ToV1(envelope)
			Expect(len(envelopes)).To(Equal(1))
			Expect(envelopes[0].GetTags()).To(HaveKeyWithValue("foo", "bar"))
		})
	})

	Context("given a v1 envelope", func() {
		It("sets v2 specific properties", func() {
			v1Envelope := &events.Envelope{
				Timestamp:  proto.Int64(99),
				Origin:     proto.String("origin-value"),
				Deployment: proto.String("some-deployment"),
				Job:        proto.String("some-job"),
				Index:      proto.String("some-index"),
				Ip:         proto.String("some-ip"),
				Tags: map[string]string{
					"random-tag": "random-value",
				},
			}

			expectedV2Envelope := &v2.Envelope{
				Timestamp: 99,
				SourceId:  "some-deployment/some-job",
				DeprecatedTags: map[string]*v2.Value{
					"random-tag": ValueText("random-value"),
					"origin":     ValueText("origin-value"),
					"deployment": ValueText("some-deployment"),
					"job":        ValueText("some-job"),
					"index":      ValueText("some-index"),
					"ip":         ValueText("some-ip"),
				},
			}

			converted := conversion.ToV2(v1Envelope, false)

			Expect(*converted).To(MatchFields(IgnoreExtras, Fields{
				"SourceId":  Equal(expectedV2Envelope.SourceId),
				"Timestamp": Equal(expectedV2Envelope.Timestamp),
			}))
			Expect(converted.DeprecatedTags["random-tag"]).To(Equal(expectedV2Envelope.DeprecatedTags["random-tag"]))
			Expect(converted.DeprecatedTags["origin"]).To(Equal(expectedV2Envelope.DeprecatedTags["origin"]))
			Expect(converted.DeprecatedTags["deployment"]).To(Equal(expectedV2Envelope.DeprecatedTags["deployment"]))
			Expect(converted.DeprecatedTags["job"]).To(Equal(expectedV2Envelope.DeprecatedTags["job"]))
			Expect(converted.DeprecatedTags["index"]).To(Equal(expectedV2Envelope.DeprecatedTags["index"]))
			Expect(converted.DeprecatedTags["ip"]).To(Equal(expectedV2Envelope.DeprecatedTags["ip"]))
		})

		It("sets non-deprecated tags", func() {
			v1 := &events.Envelope{
				Timestamp:  proto.Int64(99),
				Origin:     proto.String("origin-value"),
				Deployment: proto.String("some-deployment"),
				Job:        proto.String("some-job"),
				Index:      proto.String("some-index"),
				Ip:         proto.String("some-ip"),
				Tags: map[string]string{
					"random-tag": "random-value",
					"origin":     "origin-value",
					"deployment": "some-deployment",
					"job":        "some-job",
					"index":      "some-index",
					"ip":         "some-ip",
				},
			}
			converted := conversion.ToV2(v1, true)
			Expect(converted.Tags["random-tag"]).To(Equal(v1.Tags["random-tag"]))
			Expect(converted.Tags["origin"]).To(Equal(v1.Tags["origin"]))
			Expect(converted.Tags["deployment"]).To(Equal(v1.Tags["deployment"]))
			Expect(converted.Tags["job"]).To(Equal(v1.Tags["job"]))
			Expect(converted.Tags["index"]).To(Equal(v1.Tags["index"]))
			Expect(converted.Tags["ip"]).To(Equal(v1.Tags["ip"]))
		})
	})
})
