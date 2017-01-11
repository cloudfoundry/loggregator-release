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

var _ = Describe("ValueMetric", func() {
	Context("given a v3 envelope", func() {
		It("converts to a v2 protobuf", func() {
			envelope := &v2.Envelope{
				Message: &v2.Envelope_Gauge{
					Gauge: &v2.Gauge{
						Metrics: map[string]*v2.GaugeValue{
							"name": &v2.GaugeValue{
								Unit:  "meters",
								Value: 123,
							},
						},
					},
				},
			}

			Expect(*conversion.ToV1(envelope)).To(MatchFields(IgnoreExtras, Fields{
				"EventType": Equal(events.Envelope_ValueMetric.Enum()),
				"ValueMetric": Equal(&events.ValueMetric{
					Name:  proto.String("name"),
					Unit:  proto.String("meters"),
					Value: proto.Float64(123),
				}),
			}))
		})
	})
})
