package tagger_test

import (
	"metron/tagger"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	"github.com/pivotal-golang/localip"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"metron/envelopewriter"
)

var _ = Describe("Tagger", func() {
	It("tags events with the given deployment name, job, index and IP address", func() {
		mockWriter := &envelopewriter.MockEnvelopeWriter{}
		t := tagger.New("test-deployment", "test-job", 2, mockWriter)

		envelope := basicHttpStartStopMessage()
		t.Write(envelope)

		Expect(mockWriter.Events).To(HaveLen(1))
		expectedEnvelope := basicTaggedHttpStartStopMessage(*envelope)
		Eventually(mockWriter.Events[0]).Should(Equal(expectedEnvelope))
	})
})

func basicHttpStartStopMessage() *events.Envelope {
	return &events.Envelope{
		EventType: events.Envelope_HttpStartStop.Enum(),
		HttpStartStop: &events.HttpStartStop{
			StartTimestamp: proto.Int64(1234),
			StopTimestamp:  proto.Int64(5555),
			RequestId: &events.UUID{
				Low:  proto.Uint64(11),
				High: proto.Uint64(12),
			},
			PeerType:      events.PeerType_Server.Enum(),
			Method:        events.Method_GET.Enum(),
			Uri:           proto.String("http://test.example.com"),
			RemoteAddress: proto.String("http://test.example.com"),
			UserAgent:     proto.String("test"),
			StatusCode:    proto.Int32(1234),
			ContentLength: proto.Int64(5678),
			ApplicationId: &events.UUID{
				Low:  proto.Uint64(11),
				High: proto.Uint64(12),
			},
		},
	}
}

func basicTaggedHttpStartStopMessage(envelope events.Envelope) *events.Envelope {
	ip, _ := localip.LocalIP()

	envelope.Deployment = proto.String("test-deployment")
	envelope.Job = proto.String("test-job")
	envelope.Index = proto.String("2")
	envelope.Ip = proto.String(ip)

	return &envelope
}
