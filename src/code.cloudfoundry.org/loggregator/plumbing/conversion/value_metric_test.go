package conversion_test

import (
	"code.cloudfoundry.org/loggregator/plumbing/conversion"
	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("ValueMetric", func() {
	Context("given a v2 envelope", func() {
		It("converts to a v1 envelope", func() {
			envelope := &v2.Envelope{
				Message: &v2.Envelope_Gauge{
					Gauge: &v2.Gauge{
						Metrics: map[string]*v2.GaugeValue{
							"name": {
								Unit:  "meters",
								Value: 123,
							},
						},
					},
				},
			}

			envelopes := conversion.ToV1(envelope)
			Expect(len(envelopes)).To(Equal(1))

			converted := envelopes[0]
			Expect(*converted).To(MatchFields(IgnoreExtras, Fields{
				"EventType": Equal(events.Envelope_ValueMetric.Enum()),
				"ValueMetric": Equal(&events.ValueMetric{
					Name:  proto.String("name"),
					Unit:  proto.String("meters"),
					Value: proto.Float64(123),
				}),
			}))
		})

		It("converts multiple Gauge values", func() {
			envelope := &v2.Envelope{
				Message: &v2.Envelope_Gauge{
					Gauge: &v2.Gauge{
						Metrics: map[string]*v2.GaugeValue{
							"name": {
								Unit:  "meters",
								Value: 123,
							},
							"other-name": {
								Unit:  "feet",
								Value: 321,
							},
						},
					},
				},
			}

			envelopes := conversion.ToV1(envelope)
			Expect(len(envelopes)).To(Equal(2))

			Expect(envelopes[0].GetValueMetric().GetName()).To(Or(
				Equal("name"),
				Equal("other-name"),
			))

			Expect(envelopes[1].GetValueMetric().GetName()).To(Or(
				Equal("name"),
				Equal("other-name"),
			))

			Expect(envelopes[0].GetValueMetric().GetName()).ToNot(Equal(
				envelopes[1].GetValueMetric().GetName()))
		})

		It("is resilient to parial envelopes", func() {
			envelope := &v2.Envelope{
				Message: &v2.Envelope_Gauge{
					Gauge: &v2.Gauge{
						Metrics: map[string]*v2.GaugeValue{
							"name": nil,
						},
					},
				},
			}
			Expect(conversion.ToV1(envelope)).To(BeNil())
		})
	})

	Context("given a v1 envelope", func() {
		It("converts to a v2 envelope", func() {
			v1Envelope := &events.Envelope{
				EventType: events.Envelope_ValueMetric.Enum(),
				ValueMetric: &events.ValueMetric{
					Name:  proto.String("name"),
					Unit:  proto.String("meters"),
					Value: proto.Float64(123),
				},
			}
			expectedV2Envelope := &v2.Envelope{
				Message: &v2.Envelope_Gauge{
					Gauge: &v2.Gauge{
						Metrics: map[string]*v2.GaugeValue{
							"name": {
								Unit:  "meters",
								Value: 123,
							},
						},
					},
				},
			}
			Expect(*conversion.ToV2(v1Envelope, false)).To(MatchFields(IgnoreExtras, Fields{
				"Message": Equal(expectedV2Envelope.Message),
			}))
		})
	})
})
