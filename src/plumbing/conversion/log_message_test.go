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

var _ = Describe("LogMessage", func() {
	Context("given a v2 envelope", func() {
		It("converts messages to a v1 envelope", func() {
			envelope := &loggregator_v2.Envelope{
				Timestamp:  99,
				SourceId:   "uuid",
				InstanceId: "test-source-instance",
				DeprecatedTags: map[string]*loggregator_v2.Value{
					"source_type": {Data: &loggregator_v2.Value_Text{"test-source-type"}},
				},
				Message: &loggregator_v2.Envelope_Log{
					Log: &loggregator_v2.Log{
						Payload: []byte("Hello World"),
						Type:    loggregator_v2.Log_OUT,
					},
				},
			}

			envelopes := conversion.ToV1(envelope)
			Expect(len(envelopes)).To(Equal(1))
			oldEnvelope := envelopes[0]

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

			expectedV2Envelope := &loggregator_v2.Envelope{
				SourceId:   "uuid",
				InstanceId: "test-source-instance",
				DeprecatedTags: map[string]*loggregator_v2.Value{
					"__v1_type":   {Data: &loggregator_v2.Value_Text{"LogMessage"}},
					"source_type": {Data: &loggregator_v2.Value_Text{"test-source-type"}},
					"origin":      {Data: &loggregator_v2.Value_Text{"some-origin"}},
					"deployment":  {Data: &loggregator_v2.Value_Text{"some-deployment"}},
					"job":         {Data: &loggregator_v2.Value_Text{"some-job"}},
					"index":       {Data: &loggregator_v2.Value_Text{"some-index"}},
					"ip":          {Data: &loggregator_v2.Value_Text{"some-ip"}},
				},
				Message: &loggregator_v2.Envelope_Log{
					Log: &loggregator_v2.Log{
						Payload: []byte("Hello World"),
						Type:    loggregator_v2.Log_OUT,
					},
				},
			}

			v2Envelope := conversion.ToV2(v1Envelope, false)
			Expect(*v2Envelope).To(MatchFields(IgnoreExtras, Fields{
				"SourceId":       Equal(expectedV2Envelope.SourceId),
				"DeprecatedTags": Equal(expectedV2Envelope.DeprecatedTags),
				"Message":        Equal(expectedV2Envelope.Message),
			}))
		})

		It("sets the source ID to deployment/job when App ID is missing", func() {
			v1Envelope := &events.Envelope{
				Deployment: proto.String("some-deployment"),
				Job:        proto.String("some-job"),
				EventType:  events.Envelope_LogMessage.Enum(),
				LogMessage: &events.LogMessage{},
			}

			expectedV2Envelope := &loggregator_v2.Envelope{
				SourceId: "some-deployment/some-job",
			}

			converted := conversion.ToV2(v1Envelope, false)

			Expect(*converted).To(MatchFields(IgnoreExtras, Fields{
				"SourceId": Equal(expectedV2Envelope.SourceId),
			}))
		})
	})
})
