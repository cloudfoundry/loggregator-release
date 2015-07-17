package messagegenerator_test

import (
	"tools/benchmark/messagegenerator"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/gogo/protobuf/proto"
	"github.com/cloudfoundry/sonde-go/events"
)

var _ = Describe("Messagegenerator", func() {
	Context("The ValueMetricGenerator", func() {
		It("generates a value metric message", func() {
			generator := messagegenerator.NewValueMetricGenerator()
			bytes := generator.Generate()
			var envelope events.Envelope
			err := proto.Unmarshal(bytes, &envelope)
			Expect(err).ToNot(HaveOccurred())
			Expect(envelope.GetEventType()).To(Equal(events.Envelope_ValueMetric))
		})
	})
})
