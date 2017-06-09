package health_test

import (
	"fmt"

	"code.cloudfoundry.org/loggregator/metron/internal/health"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Registry", func() {
	It("returns an existing value if one already exists", func() {
		registry := health.NewRegistry()

		initialValue := registry.RegisterValue("any_value")
		initialValue.Increment(5)

		refetchValue := registry.RegisterValue("any_value")
		Expect(refetchValue.Number()).To(Equal(int64(5)))
	})

	It("is thread safe", func() {
		registry := health.NewRegistry()

		go func() {
			for i := 0; i < 10000; i++ {
				registry.RegisterValue(fmt.Sprint("any_value_", i))
			}
		}()

		for i := 0; i < 10000; i++ {
			registry.RegisterValue("any_value")
		}

		// Expect no data race to occur
	})

	It("returns its current state", func() {
		registry := health.NewRegistry()

		registry.RegisterValue("any_value")

		Expect(registry.State()).To(Equal(map[string]int64{
			"any_value": 0,
		}))
	})
})
