package messagegenerator_test

import (
	"tools/benchmark/messagegenerator"

	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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

	Context("The LogGenerator", func() {
		It("generates a log message envelope", func() {
			generator := messagegenerator.NewLogMessageGenerator("appID")
			bytes := generator.Generate()
			var envelope events.Envelope
			err := proto.Unmarshal(bytes, &envelope)
			Expect(err).ToNot(HaveOccurred())
			Expect(envelope.GetLogMessage()).ToNot(BeNil())
			Expect(envelope.GetLogMessage().GetMessage()).To(BeEquivalentTo("test message"))
		})
	})

	Context("The LegacyLogGenerator", func() {
		It("generates a legacy log message envelope", func() {
			generator := messagegenerator.NewLegacyLogGenerator()
			bytes := generator.Generate()
			var envelope logmessage.LogEnvelope
			err := proto.Unmarshal(bytes, &envelope)
			Expect(err).ToNot(HaveOccurred())
			Expect(envelope.GetLogMessage()).ToNot(BeNil())
			Expect(envelope.GetLogMessage().GetMessage()).To(BeEquivalentTo("test message"))
		})
	})
})
