package tagger_test

import (
	"metron/writers/tagger"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	"code.cloudfoundry.org/localip"

	"metron/writers/mocks"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tagger", func() {
	It("tags events with the given deployment name, job, index and IP address", func() {
		mockWriter := &mocks.MockEnvelopeWriter{}
		t := tagger.New("test-deployment", "test-job", "2", mockWriter)

		envelope := basicMessage()
		t.Write(envelope)

		Expect(mockWriter.Events).To(HaveLen(1))
		expectedEnvelope := basicTaggedMessage(*envelope)

		Eventually(mockWriter.Events[0]).Should(Equal(expectedEnvelope))
	})

	Context("doesn't overwrite", func() {
		var mockWriter *mocks.MockEnvelopeWriter
		var t *tagger.Tagger
		var envelope *events.Envelope

		BeforeEach(func() {
			mockWriter = &mocks.MockEnvelopeWriter{}
			t = tagger.New("test-deployment", "test-job", "2", mockWriter)

			envelope = basicMessage()
		})

		It("when deployment is already set", func() {
			envelope.Deployment = proto.String("another-deployment")
			t.Write(envelope)

			Expect(mockWriter.Events).To(HaveLen(1))
			writtenEnvelope := mockWriter.Events[0]
			Eventually(*writtenEnvelope.Deployment).Should(Equal("another-deployment"))
		})

		It("when job is already set", func() {
			envelope.Job = proto.String("another-job")
			t.Write(envelope)

			Expect(mockWriter.Events).To(HaveLen(1))
			writtenEnvelope := mockWriter.Events[0]
			Eventually(*writtenEnvelope.Job).Should(Equal("another-job"))
		})

		It("when index is already set", func() {
			envelope.Index = proto.String("3")
			t.Write(envelope)

			Expect(mockWriter.Events).To(HaveLen(1))
			writtenEnvelope := mockWriter.Events[0]
			Eventually(*writtenEnvelope.Index).Should(Equal("3"))
		})

		It("when ip is already set", func() {
			envelope.Ip = proto.String("1.1.1.1")
			t.Write(envelope)

			Expect(mockWriter.Events).To(HaveLen(1))
			writtenEnvelope := mockWriter.Events[0]
			Eventually(*writtenEnvelope.Ip).Should(Equal("1.1.1.1"))
		})
	})
})

func basicMessage() *events.Envelope {
	return &events.Envelope{
		EventType: events.Envelope_ValueMetric.Enum(),
		ValueMetric: &events.ValueMetric{
			Name:  proto.String("metricName"),
			Value: proto.Float64(2.0),
			Unit:  proto.String("seconds"),
		},
	}
}

func basicTaggedMessage(envelope events.Envelope) *events.Envelope {
	ip, _ := localip.LocalIP()

	envelope.Deployment = proto.String("test-deployment")
	envelope.Job = proto.String("test-job")
	envelope.Index = proto.String("2")
	envelope.Ip = proto.String(ip)

	return &envelope
}
