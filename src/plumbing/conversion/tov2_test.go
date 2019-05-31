package conversion_test

import (
	"time"

	"code.cloudfoundry.org/loggregator/plumbing/conversion"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ToV2", func() {
	It("doesn't modify the input", func() {
		tags := make(map[string]string)
		tags["origin"] = "not-doppler"

		v1e := &events.Envelope{
			Origin:    proto.String("doppler"),
			EventType: events.Envelope_LogMessage.Enum(),
			Timestamp: proto.Int64(time.Now().UnixNano()),
			LogMessage: &events.LogMessage{
				Message:     []byte("some-log-message"),
				MessageType: events.LogMessage_OUT.Enum(),
				Timestamp:   proto.Int64(time.Now().UnixNano()),
			},
			Tags: tags,
		}

		v2e := conversion.ToV2(v1e, true)

		Expect(v1e.GetTags()["origin"]).To(Equal("not-doppler"))
		Expect(v2e.GetTags()["origin"]).To(Equal("doppler"))
	})
})
