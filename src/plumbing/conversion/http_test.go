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

var _ = Describe("Http", func() {
	ValueText := func(s string) *v2.Value {
		return &v2.Value{&v2.Value_Text{Text: s}}
	}

	ValueInteger := func(i int64) *v2.Value {
		return &v2.Value{&v2.Value_Integer{Integer: i}}
	}

	Context("given a v3 envelope", func() {
		It("converts to a v2 protobuf", func() {
			envelope := &v2.Envelope{
				SourceUuid: "b3015d69-09cd-476d-aace-ad2d824d5ab7",
				Message: &v2.Envelope_Timer{
					Timer: &v2.Timer{
						Name:  "http",
						Start: 99,
						Stop:  100,
					},
				},
				Tags: map[string]*v2.Value{
					"request_id":     ValueText("954f61c4-ac84-44be-9217-cdfa3117fb41"),
					"peer_type":      ValueText("Client"),
					"method":         ValueText("GET"),
					"uri":            ValueText("/hello-world"),
					"remote_address": ValueText("10.1.1.0"),
					"user_agent":     ValueText("Mozilla/5.0"),
					"status_code":    ValueInteger(200),
					"content_length": ValueInteger(1000000),
					"instance_index": ValueInteger(10),
					"instance_id":    ValueText("application-id"),
					"forwarded":      ValueText("6.6.6.6\n8.8.8.8"),
				},
			}

			converted_envelope := conversion.ToV1(envelope)

			_, err := proto.Marshal(converted_envelope)
			Expect(err).ToNot(HaveOccurred())

			Expect(*converted_envelope).To(MatchFields(IgnoreExtras, Fields{
				"EventType": Equal(events.Envelope_HttpStartStop.Enum()),
				"HttpStartStop": Equal(&events.HttpStartStop{
					StartTimestamp: proto.Int64(99),
					StopTimestamp:  proto.Int64(100),
					RequestId: &events.UUID{
						Low:  proto.Uint64(13710229043186585493),
						High: proto.Uint64(4754419335048271762),
					},
					ApplicationId: &events.UUID{
						Low:  proto.Uint64(7874487913786704307),
						High: proto.Uint64(13211957678352223914),
					},
					PeerType:      events.PeerType_Client.Enum(),
					Method:        events.Method_GET.Enum(),
					Uri:           proto.String("/hello-world"),
					RemoteAddress: proto.String("10.1.1.0"),
					UserAgent:     proto.String("Mozilla/5.0"),
					StatusCode:    proto.Int32(200),
					ContentLength: proto.Int64(1000000),
					InstanceIndex: proto.Int32(10),
					InstanceId:    proto.String("application-id"),
					Forwarded:     []string{"6.6.6.6", "8.8.8.8"},
				}),
			}))
		})
	})
})
