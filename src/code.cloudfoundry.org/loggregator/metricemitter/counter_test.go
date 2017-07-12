package metricemitter_test

import (
	"errors"

	"code.cloudfoundry.org/loggregator/metricemitter"

	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Counter", func() {
	Context("WithEnvelope", func() {
		It("decrements it value on success", func() {
			metric := metricemitter.NewCounter("name", "source-id")

			metric.Increment(10)

			err := metric.WithEnvelope(func(_ *v2.Envelope) error {
				return nil
			})
			Expect(err).ToNot(HaveOccurred())

			Expect(metric.GetDelta()).To(Equal(uint64(0)))
		})

		It("does not decrement the value on failure", func() {
			metric := metricemitter.NewCounter("name", "source-id")

			metric.Increment(10)

			err := metric.WithEnvelope(func(_ *v2.Envelope) error {
				return errors.New("some error")
			})
			Expect(err).To(HaveOccurred())

			Expect(metric.GetDelta()).To(Equal(uint64(10)))
		})
	})
})
