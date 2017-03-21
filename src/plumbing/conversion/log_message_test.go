package conversion_test

import (
	"plumbing/conversion"
	v2 "plumbing/v2"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("LogMessage", func() {
	Context("given a v2 envelope", func() {
		It("converts messages to a v1 envelope", func() {
			envelope := &v2.Envelope{
				Timestamp: 99,
				SourceId:  "uuid",
				Tags: map[string]*v2.Value{
					"source_type":     {&v2.Value_Text{"test-source-type"}},
					"source_instance": {&v2.Value_Text{"test-source-instance"}},
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
					SourceType:     proto.String("test-source-type"),
					SourceInstance: proto.String("test-source-instance"),
				}),
			}))
		})
	})

	Context("given a v1 envelop", func() {
		It("converts messages to v2 envelopes", func() {
			v1Envelope := &events.Envelope{
				Origin:     proto.String("some-origin"),
				EventType:  events.Envelope_LogMessage.Enum(),
				Deployment: proto.String("some-deployment"),
				Job:        proto.String("some-job"),
				Index:      proto.String("some-index"),
				Ip:         proto.String("some-ip"),
				LogMessage: &events.LogMessage{
					Message:        []byte("Hello World"),
					MessageType:    events.LogMessage_OUT.Enum(),
					Timestamp:      proto.Int64(99),
					AppId:          proto.String("uuid"),
					SourceType:     proto.String("test-source-type"),
					SourceInstance: proto.String("test-source-instance"),
				},
			}

			expectedV2Envelope := &v2.Envelope{
				SourceId: "uuid",
				Tags: map[string]*v2.Value{
					"__v1_type":       {&v2.Value_Text{"LogMessage"}},
					"source_type":     {&v2.Value_Text{"test-source-type"}},
					"source_instance": {&v2.Value_Text{"test-source-instance"}},
					"origin":          {&v2.Value_Text{"some-origin"}},
					"deployment":      {&v2.Value_Text{"some-deployment"}},
					"job":             {&v2.Value_Text{"some-job"}},
					"index":           {&v2.Value_Text{"some-index"}},
					"ip":              {&v2.Value_Text{"some-ip"}},
				},
				Message: &v2.Envelope_Log{
					Log: &v2.Log{
						Payload: []byte("Hello World"),
						Type:    v2.Log_OUT,
					},
				},
			}

			v2Envelope := conversion.ToV2(v1Envelope)
			Expect(*v2Envelope).To(MatchFields(IgnoreExtras, Fields{
				"SourceId": Equal(expectedV2Envelope.SourceId),
				"Tags":     Equal(expectedV2Envelope.Tags),
				"Message":  Equal(expectedV2Envelope.Message),
			}))
		})

		It("sets the source ID to deployment/job when App ID is missing", func() {
			v1Envelope := &events.Envelope{
				Deployment: proto.String("some-deployment"),
				Job:        proto.String("some-job"),
				EventType:  events.Envelope_LogMessage.Enum(),
				LogMessage: &events.LogMessage{},
			}

			expectedV2Envelope := &v2.Envelope{
				SourceId: "some-deployment/some-job",
			}

			converted := conversion.ToV2(v1Envelope)

			Expect(*converted).To(MatchFields(IgnoreExtras, Fields{
				"SourceId": Equal(expectedV2Envelope.SourceId),
			}))
		})
	})
})
