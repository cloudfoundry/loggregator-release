package conversion_test

import (
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator/plumbing/conversion"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	goproto "github.com/golang/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("HTTP", func() {
	Context("given a v1 envelope", func() {
		It("converts to a v2 envelope", func() {
			v1Envelope := &events.Envelope{
				EventType:  events.Envelope_Error.Enum(),
				Origin:     proto.String("fake-origin"),
				Deployment: proto.String("some-deployment"),
				Job:        proto.String("some-job"),
				Index:      proto.String("some-index"),
				Ip:         proto.String("some-ip"),
				Error: &events.Error{
					Source:  proto.String("test-source"),
					Code:    proto.Int32(12345),
					Message: proto.String("test-message"),
				},
			}

			expectedV2Envelope := &loggregator_v2.Envelope{
				DeprecatedTags: map[string]*loggregator_v2.Value{
					"__v1_type":  {Data: &loggregator_v2.Value_Text{"Error"}},
					"source":     {Data: &loggregator_v2.Value_Text{"test-source"}},
					"code":       {Data: &loggregator_v2.Value_Text{"12345"}},
					"origin":     {Data: &loggregator_v2.Value_Text{"fake-origin"}},
					"deployment": {Data: &loggregator_v2.Value_Text{"some-deployment"}},
					"job":        {Data: &loggregator_v2.Value_Text{"some-job"}},
					"index":      {Data: &loggregator_v2.Value_Text{"some-index"}},
					"ip":         {Data: &loggregator_v2.Value_Text{"some-ip"}},
				},
				Message: &loggregator_v2.Envelope_Log{
					Log: &loggregator_v2.Log{
						Payload: []byte("test-message"),
						Type:    loggregator_v2.Log_OUT,
					},
				},
			}

			converted := conversion.ToV2(v1Envelope, false)

			_, err := goproto.Marshal(converted)
			Expect(err).ToNot(HaveOccurred())

			for k, v := range expectedV2Envelope.DeprecatedTags {
				Expect(goproto.Equal(converted.GetDeprecatedTags()[k], v)).To(BeTrue())
			}

			// Expect(converted.GetError().GetSource()).To(Equal(expectedV2Envelope.GetError().GetSource()))
			// Expect(converted.GetError().GetCode()).To(Equal(expectedV2Envelope.GetError().GetCode()))
			Expect(string(converted.GetLog().GetPayload())).To(Equal(string(expectedV2Envelope.GetLog().GetPayload())))
		})
	})
})
