package metric_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"metric"
	v2 "plumbing/v2"
)

var _ = Describe("default emitter", func() {
	var (
		receiver <-chan *v2.Envelope
		addr     string
	)

	Context("with a default emitter setup", func() {
		BeforeEach(func() {
			var spyIngressServer *SpyIngressServer
			addr, spyIngressServer = startIngressServer()

			metric.Setup(
				metric.WithAddr(addr),
				metric.WithBatchInterval(time.Millisecond),
			)

			metric.IncCounter("seed-data")

			rx := fetchReceiver(spyIngressServer)
			receiver = rxToCh(rx)
		})

		It("can increment a counter", func() {
			metric.IncCounter("foo", metric.WithIncrement(42))
			var e *v2.Envelope
			f := func() bool {
				Eventually(receiver).Should(Receive(&e))
				return e.GetCounter().Name == "foo"
			}
			Eventually(f).Should(BeTrue())
			Expect(e.GetCounter().GetDelta()).To(Equal(uint64(42)))
		})
	})
})
