package conversion_test

import (
	"fmt"
	"plumbing/conversion"
	v2 "plumbing/v2"

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
				Tags: map[string]*v2.Value{
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

			oldEnvelope := conversion.ToV1(envelope)
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
				Tags: map[string]*v2.Value{
					"foo": {&v2.Value_Text{"bar"}},
					"baz": nil,
				},
				Message: &v2.Envelope_Log{Log: &v2.Log{}},
			}

			oldEnvelope := conversion.ToV1(envelope)
			Expect(oldEnvelope.Tags).To(Equal(map[string]string{
				"foo": "bar",
			}))
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
				Tags: map[string]*v2.Value{
					"random-tag": ValueText("random-value"),
					"origin":     ValueText("origin-value"),
					"deployment": ValueText("some-deployment"),
					"job":        ValueText("some-job"),
					"index":      ValueText("some-index"),
					"ip":         ValueText("some-ip"),
				},
			}

			converted := conversion.ToV2(v1Envelope)

			Expect(*converted).To(MatchFields(IgnoreExtras, Fields{
				"SourceId":  Equal(expectedV2Envelope.SourceId),
				"Timestamp": Equal(expectedV2Envelope.Timestamp),
			}))
			Expect(converted.Tags["random-tag"]).To(Equal(expectedV2Envelope.Tags["random-tag"]))
			Expect(converted.Tags["origin"]).To(Equal(expectedV2Envelope.Tags["origin"]))
			Expect(converted.Tags["deployment"]).To(Equal(expectedV2Envelope.Tags["deployment"]))
			Expect(converted.Tags["job"]).To(Equal(expectedV2Envelope.Tags["job"]))
			Expect(converted.Tags["index"]).To(Equal(expectedV2Envelope.Tags["index"]))
			Expect(converted.Tags["ip"]).To(Equal(expectedV2Envelope.Tags["ip"]))
		})
	})
})
