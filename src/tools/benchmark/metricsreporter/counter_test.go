package metricsreporter_test

import (
	"tools/benchmark/metricsreporter"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Counter", func() {
	var counter *metricsreporter.Counter

	BeforeEach(func() {
		counter = metricsreporter.NewCounter("counter")
	})

	Context("GetName", func() {
		It("returns its name", func() {
			Expect(counter.GetName()).To(Equal("counter"))
		})
	})

	Context("GetValue", func() {
		It("returns its current value", func() {
			Expect(counter.GetValue()).To(BeEquivalentTo(0))
		})
	})

	Context("GetTotal", func() {
		It("returns its current total", func() {
			Expect(counter.GetTotal()).To(BeEquivalentTo(0))
		})
	})

	Context("IncrementValue", func() {
		It("increments the value", func() {
			counter.IncrementValue()
			Expect(counter.GetValue()).To(BeEquivalentTo(1))
		})
	})

	Context("Reset", func() {
		It("adds the current value to the total and resets the value to zero", func() {
			counter.IncrementValue()
			counter.IncrementValue()

			counter.Reset()

			Expect(counter.GetValue()).To(BeEquivalentTo(0))
			Expect(counter.GetTotal()).To(BeEquivalentTo(2))
		})
	})
})
