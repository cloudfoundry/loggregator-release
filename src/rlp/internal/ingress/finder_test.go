package ingress_test

import (
	"rlp/internal/ingress"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Finder", func() {
	It("returns a doppler service event of all the dopplers", func() {
		finder := ingress.NewFinder([]string{"1.1.1.1", "2.2.2.2"})
		event := finder.Next()

		Expect(event.GRPCDopplers).To(Equal([]string{"1.1.1.1", "2.2.2.2"}))
	})

	It("blocks after the first call", func() {
		finder := ingress.NewFinder([]string{"1.1.1.1", "2.2.2.2"})
		finder.Next()

		done := make(chan struct{})
		go func() {
			defer close(done)
			finder.Next()
		}()

		Consistently(done).Should(Not(BeClosed()))
	})
})
