package health_test

import (
	"code.cloudfoundry.org/loggregator/metron/internal/health"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Value", func() {
	It("increments the value", func() {
		value := &health.Value{}

		value.Increment(3)

		Expect(value.Number()).To(Equal(int64(3)))
	})

	It("decrements the value", func() {
		value := &health.Value{}

		value.Decrement(3)

		Expect(value.Number()).To(Equal(int64(-3)))
	})
})
