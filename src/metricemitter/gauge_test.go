package metricemitter_test

import (
	"code.cloudfoundry.org/go-loggregator/v10/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-release/src/metricemitter"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Gauge", func() {
	It("creates a gauge envelope with the expected value", func() {
		metric := metricemitter.NewGauge("name", "unit", "source-id",
			metricemitter.WithTags(map[string]string{
				"a": "1",
			}),
		)
		metric.Set(99.9)

		var actualEnv *loggregator_v2.Envelope
		err := metric.WithEnvelope(func(env *loggregator_v2.Envelope) error {
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

	It("increments the value", func() {
		metric := metricemitter.NewGauge("name", "unit", "source-id",
			metricemitter.WithTags(map[string]string{
				"a": "1",
			}),
		)

		metric.Set(99.9)
		metric.Increment(1.1)

		Expect(metric.GetValue()).To(Equal(101.0))
	})

	It("decrements the value", func() {
		metric := metricemitter.NewGauge("name", "unit", "source-id",
			metricemitter.WithTags(map[string]string{
				"a": "1",
			}),
		)

		metric.Set(99.9)
		metric.Decrement(1.1)

		Expect(metric.GetValue()).To(Equal(98.8))
	})
})
