package metricemitter_test

import (
	"code.cloudfoundry.org/loggregator/metricemitter"

	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Gauge", func() {
	Context("WithEnvelope", func() {
		It("creates a gauge envelope with the expected value", func() {
			metric := metricemitter.NewGauge("name", "unit", "source-id",
				metricemitter.WithTags(map[string]string{
					"a": "1",
				}))
			metric.Set(99.9)

			var actualEnv *v2.Envelope
			err := metric.WithEnvelope(func(env *v2.Envelope) error {
				actualEnv = env
				return nil
			})
			Expect(err).ToNot(HaveOccurred())

			Expect(actualEnv.GetSourceId()).To(Equal("source-id"))
			Expect(actualEnv.GetTimestamp()).ToNot(BeZero())
			metrics := actualEnv.GetGauge().GetMetrics()
			Expect(metrics).To(HaveLen(1))
			Expect(metrics["name"].GetUnit()).To(Equal("unit"))
			Expect(metrics["name"].GetValue()).To(Equal(99.9))
			Expect(actualEnv.GetDeprecatedTags()).To(HaveLen(1))
			Expect(actualEnv.GetDeprecatedTags()["a"].GetText()).To(Equal("1"))
		})
	})
})
