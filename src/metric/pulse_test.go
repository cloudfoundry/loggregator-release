package metric_test

// import (
// 	. "github.com/onsi/ginkgo"
// 	. "github.com/onsi/gomega"
// )

// var _ = Describe("Pulse", func() {
// 	var (
// 		emitter          *metric.Emitter
// 		spyIngressServer *SpyIngressServer
// 		receiver         <-chan *v2.Envelope
// 		cleanup          func()
// 	)

// 	BeforeEach(func() {
// 		var (
// 			addr string
// 			err  error
// 		)
// 		addr, spyIngressServer, cleanup = startIngressServer()
// 		emitter, err = metric.New(
// 			metric.WithAddr(addr),
// 			metric.WithSourceID("some-uuid"),
// 			metric.WithRepeatInterval(250*time.Millisecond),
// 		)
// 		Expect(err).ToNot(HaveOccurred())

// 		emitter.IncCounter("seed-data")

// 		rx := fetchReceiver(spyIngressServer)
// 		receiver = rxToCh(rx)
// 	})

// 	It("emits a counter event periodically to the consumer", func() {
// 		emitter.Repeat(
// 			"some-metric",
// 			WithTag("foo", "bar"),
// 			WithInterval(10*time.Millisecond),
// 		)
// 		var e *v2.Envelope
// 		f := func() bool {
// 			Eventually(receiver).Should(Receive(&e))

// 			counter := e.GetCounter()
// 			if counter == nil {
// 				return false
// 			}

// 			return counter.Name == "repeated-metric"
// 		}

// 		for i := 0; i < 3; i++ {
// 			Eventually(receiver).Should(Receive(&e))
// 		}

// 		Eventually(f).Should(BeTrue())
// 		Expect(e.Timestamp).ToNot(Equal(int64(0)))
// 		Expect(e.SourceId).To(Equal("some-uuid"))
// 		Expect(e.GetCounter().GetDelta()).To(Equal(uint64(5)))

// 		Expect(e.GetTags()["origin"].GetText()).To(Equal("loggregator.metron"))
// 	})
// })
