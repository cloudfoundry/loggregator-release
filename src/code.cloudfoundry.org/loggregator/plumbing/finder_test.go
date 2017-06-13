package plumbing_test

import (
	"code.cloudfoundry.org/loggregator/plumbing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("StaticFinder", func() {
	It("returns a doppler service event of all the dopplers", func() {
		finder := plumbing.NewStaticFinder([]string{"1.1.1.1", "2.2.2.2"})
		event := finder.Next()

		Expect(event.GRPCDopplers).To(Equal([]string{"1.1.1.1", "2.2.2.2"}))
	})

	It("blocks after the first call", func() {
		finder := plumbing.NewStaticFinder([]string{"1.1.1.1", "2.2.2.2"})
		finder.Next()

		done := make(chan struct{})
		go func() {
			defer close(done)
			finder.Next()
		}()

		Consistently(done).Should(Not(BeClosed()))
	})

	It("return no dopplers after Stoping", func() {
		finder := plumbing.NewStaticFinder([]string{"1.1.1.1", "2.2.2.2"})
		finder.Next()
		finder.Stop()
		event := finder.Next()

		Expect(event.GRPCDopplers).To(HaveLen(0))
	})
})
