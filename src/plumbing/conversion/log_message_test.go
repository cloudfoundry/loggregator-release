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

var _ = Describe("LogMessage", func() {
	Context("given a v3 envelope", func() {
		Context("for stderr", func() {
			It("converts it to a v2 protobuf", func() {
				envelope := &v2.Envelope{
					Timestamp: 99,
					SourceId:  "uuid",
					Tags: map[string]*v2.Value{
						"source_type":     {&v2.Value_Text{"value"}},
						"source_instance": {&v2.Value_Text{"value"}},
						"origin":          {&v2.Value_Text{"value"}},
					},
					Message: &v2.Envelope_Log{
						Log: &v2.Log{
							Payload: []byte("Hello World"),
							Type:    v2.Log_ERR,
						},
					},
				}

				oldEnvelope := conversion.ToV1(envelope)
				Expect(*oldEnvelope).To(MatchFields(IgnoreExtras, Fields{
					"EventType": Equal(events.Envelope_LogMessage.Enum()),
					"LogMessage": Equal(&events.LogMessage{
						Message:        []byte("Hello World"),
						MessageType:    events.LogMessage_ERR.Enum(),
						Timestamp:      proto.Int64(99),
						AppId:          proto.String("uuid"),
						SourceType:     proto.String("value"),
						SourceInstance: proto.String("value"),
					}),
				}))
			})
		})
		Context("for stdout", func() {
			It("converts it to a v2 protobuf", func() {
				envelope := &v2.Envelope{
					Timestamp: 99,
					SourceId:  "uuid",
					Tags: map[string]*v2.Value{
						"source_type":     {&v2.Value_Text{"value"}},
						"source_instance": {&v2.Value_Text{"value"}},
						"origin":          {&v2.Value_Text{"value"}},
					},
					Message: &v2.Envelope_Log{
						Log: &v2.Log{
							Payload: []byte("Hello World"),
							Type:    v2.Log_OUT,
						},
					},
				}

				oldEnvelope := conversion.ToV1(envelope)
				Expect(*oldEnvelope).To(MatchFields(IgnoreExtras, Fields{
					"EventType": Equal(events.Envelope_LogMessage.Enum()),
					"LogMessage": Equal(&events.LogMessage{
						Message:        []byte("Hello World"),
						MessageType:    events.LogMessage_OUT.Enum(),
						Timestamp:      proto.Int64(99),
						AppId:          proto.String("uuid"),
						SourceType:     proto.String("value"),
						SourceInstance: proto.String("value"),
					}),
				}))
			})
		})

		Context("for v1 envelope specific properties", func() {
			It("sets them", func() {
				envelope := &v2.Envelope{
					Timestamp: 99,
					SourceId:  "uuid",
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
		})
	})
})
