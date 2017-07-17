package plumbing_test

import (
	"time"

	"code.cloudfoundry.org/loggregator/plumbing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EnvelopeAverager", func() {
	var (
		e *plumbing.EnvelopeAverager
		c chan float64
	)

	var setupReceiver = func(c chan<- float64) func(float64) {
		return func(f float64) {
			c <- f
		}
	}

	BeforeEach(func() {
		e = plumbing.NewEnvelopeAverager()
		c = make(chan float64)
	})

	It("emits a 0 on an interval when no data is given", func() {
		e.Start(time.Millisecond, setupReceiver(c))
		Eventually(c).Should(Receive(Equal(0.0)))
	})

	It("emits an average at the given interval", func() {
		e.Start(time.Millisecond, setupReceiver(c))
		e.Track(1, 100)
		Eventually(c).Should(Receive(Equal(100.0)))
	})

	It("resets its average each tick", func() {
		e.Start(time.Millisecond, setupReceiver(c))
		e.Track(1, 100)
		Eventually(c).Should(Receive(Equal(100.0)))
		Eventually(c).Should(Receive(Equal(0.0)))
	})

	It("handles an overflow", func() {
		e.Track(0xFFFF, 0xFFFFF)
		e.Start(time.Millisecond, setupReceiver(c))
		Eventually(c).Should(Receive(BeNumerically("~", 16.0, 0.1)))

		e.Track(1, 1)
		Eventually(c).Should(Receive(Equal(1.0)))
	})
})
