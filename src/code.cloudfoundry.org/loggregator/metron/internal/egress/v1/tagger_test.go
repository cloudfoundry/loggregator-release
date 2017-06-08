package v1_test

import (
	egress "code.cloudfoundry.org/loggregator/metron/internal/egress/v1"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tagger", func() {
	It("tags events with the given deployment name, job, index and IP address", func() {
		mockWriter := &MockEnvelopeWriter{}
		t := egress.NewTagger(
			"test-deployment",
			"test-job",
			"2",
			"123.123.123.123",
			mockWriter,
		)
		envelope := &events.Envelope{
			EventType: events.Envelope_ValueMetric.Enum(),
			ValueMetric: &events.ValueMetric{
				Name:  proto.String("metricName"),
				Value: proto.Float64(2.0),
				Unit:  proto.String("seconds"),
			},
		}

		t.Write(envelope)

		Expect(mockWriter.Events).To(HaveLen(1))
		expected := &events.Envelope{
			EventType: events.Envelope_ValueMetric.Enum(),
			ValueMetric: &events.ValueMetric{
				Name:  proto.String("metricName"),
				Value: proto.Float64(2.0),
				Unit:  proto.String("seconds"),
			},
			Deployment: proto.String("test-deployment"),
			Job:        proto.String("test-job"),
			Index:      proto.String("2"),
			Ip:         proto.String("123.123.123.123"),
		}
		Eventually(mockWriter.Events[0]).Should(Equal(expected))
	})

	Context("doesn't overwrite", func() {
		var (
			mockWriter *MockEnvelopeWriter
			t          *egress.Tagger
			envelope   *events.Envelope
		)

		BeforeEach(func() {
			mockWriter = &MockEnvelopeWriter{}
			t = egress.NewTagger(
				"test-deployment",
				"test-job",
				"2",
				"123.123.123.123",
				mockWriter,
			)

			envelope = &events.Envelope{
				EventType: events.Envelope_ValueMetric.Enum(),
				ValueMetric: &events.ValueMetric{
					Name:  proto.String("metricName"),
					Value: proto.Float64(2.0),
					Unit:  proto.String("seconds"),
				},
			}
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
