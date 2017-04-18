package metric_test

import (
	"fmt"
	"math/rand"
	"metric"
	"time"

	v2 "plumbing/v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Metric", func() {
	var (
		emitter  *metric.Emitter
		receiver <-chan *v2.Envelope
	)

	BeforeEach(func() {
		var (
			addr             string
			spyIngressServer *SpyIngressServer
		)
		addr, spyIngressServer = startIngressServer()
		var err error
		emitter, err = metric.New(
			metric.WithAddr(addr),
			metric.WithSourceID("some-uuid"),
			metric.WithBatchInterval(time.Millisecond),
			metric.WithOrigin("loggregator.metron"),
			metric.WithDeploymentMeta("some-deployment", "some-job", "some-index"),
		)
		Expect(err).ToNot(HaveOccurred())

		emitter.IncCounter("seed-data")

		rx := fetchReceiver(spyIngressServer)
		receiver = rxToCh(rx)
		Eventually(receiver).Should(Receive())
	})

	Context("when a consumer is not available", func() {
		It("does not block when emitting metric", func() {
			done := make(chan struct{})
			emitter, err := metric.New(
				metric.WithAddr("does-not-exist:0"),
			)
			Expect(err).ToNot(HaveOccurred())
			go func() {
				defer close(done)
				emitter.IncCounter("some-name")
			}()
			Eventually(done).Should(BeClosed())
		})
	})

	Describe("IncCounter()", func() {
		It("increments a counter event emitted to the consumer", func() {
			randName := generateRandName()
			for i := 0; i < 5; i++ {
				emitter.IncCounter(randName)
			}

			var e *v2.Envelope
			f := func() bool {
				Eventually(receiver).Should(Receive(&e))

				counter := e.GetCounter()
				if counter == nil {
					return false
				}

				return counter.Name == randName
			}
			Eventually(f).Should(BeTrue())
			Expect(e.Timestamp).ToNot(Equal(int64(0)))
			Expect(e.SourceId).To(Equal("some-uuid"))
			Expect(e.GetCounter().GetDelta()).To(Equal(uint64(5)))

			Expect(e.GetTags()["origin"].GetText()).To(Equal("loggregator.metron"))
		})

		It("increments by the given value", func() {
			randName := generateRandName()
			emitter.IncCounter(randName)
			emitter.IncCounter(randName, metric.WithIncrement(42))

			var e *v2.Envelope
			f := func() bool {
				Eventually(receiver).Should(Receive(&e))

				counter := e.GetCounter()
				if counter == nil {
					return false
				}

				return counter.Name == randName
			}

			Eventually(f).Should(BeTrue())
			Expect(e.GetCounter().GetDelta()).To(Equal(uint64(43)))
		})

		It("tags with the given version", func() {
			randName := generateRandName()
			emitter.IncCounter(randName, metric.WithVersion(1, 2))
			var e *v2.Envelope
			f := func() bool {
				Eventually(receiver).Should(Receive(&e))

				counter := e.GetCounter()
				if counter == nil {
					return false
				}

				return counter.Name == randName
			}
			Eventually(f).Should(BeTrue())
			Expect(e.GetTags()["metric_version"].GetText()).To(Equal("1.2"))
		})

		It("adds additional tags", func() {
			randName := generateRandName()
			emitter.IncCounter(randName, metric.WithTag("name", "value"))
			var e *v2.Envelope
			f := func() bool {
				Eventually(receiver).Should(Receive(&e))

				counter := e.GetCounter()
				if counter == nil {
					return false
				}

				return counter.Name == randName
			}
			Eventually(f).Should(BeTrue())
			Expect(e.GetTags()["name"].GetText()).To(Equal("value"))
		})

		It("tags with meta deployment tags", func() {
			randName := generateRandName()
			emitter.IncCounter(randName, metric.WithIncrement(42))
			var e *v2.Envelope
			f := func() bool {
				Eventually(receiver).Should(Receive(&e))

				counter := e.GetCounter()
				if counter == nil {
					return false
				}

				return counter.Name == randName
			}

			Eventually(f).Should(BeTrue())
			Expect(e.Tags["deployment"].GetText()).To(Equal("some-deployment"))
			Expect(e.Tags["job"].GetText()).To(Equal("some-job"))
			Expect(e.Tags["index"].GetText()).To(Equal("some-index"))
		})
	})

	Describe("Pulse()", func() {
		It("pulses zero values for metrics that are not incremented", func() {
			randName := generateRandName()
			emitter.PulseCounter(
				randName,
				metric.WithPulseInterval(time.Millisecond),
			)

			for i := 0; i < 3; i++ {
				var e *v2.Envelope
				Eventually(receiver).Should(Receive(&e))
				Expect(e.GetCounter().GetDelta()).To(BeZero())
			}
		})

		It("returns a func that can increment the counter", func() {
			randName := generateRandName()
			increment := emitter.PulseCounter(
				randName,
				metric.WithPulseInterval(time.Millisecond),
			)

			var e *v2.Envelope
			Eventually(receiver).Should(Receive(&e))
			Expect(e.GetCounter().GetDelta()).To(BeZero())

			increment(42)
			f := func() bool {
				var e *v2.Envelope
				Eventually(receiver).Should(Receive(&e))
				return e.GetCounter().GetDelta() == 42
			}
			Eventually(f).Should(BeTrue())
		})

		It("pulses tags", func() {
			randName := generateRandName()
			emitter.PulseCounter(
				randName,
				metric.WithPulseTag("foo", "bar"),
				metric.WithPulseInterval(time.Millisecond),
			)

			var e *v2.Envelope
			Eventually(receiver).Should(Receive(&e))
			Expect(e.GetCounter().GetDelta()).To(BeZero())
			Expect(e.Tags["foo"].GetText()).To(Equal("bar"))
		})

		It("increments with tags", func() {
			randName := generateRandName()
			increment := emitter.PulseCounter(
				randName,
				metric.WithPulseTag("foo", "baz"),
			)

			increment(42)
			var e *v2.Envelope
			f := func() bool {
				Eventually(receiver).Should(Receive(&e))
				return e.GetCounter().GetDelta() == 42
			}
			Eventually(f).Should(BeTrue())
			Expect(e.Tags["foo"].GetText()).To(Equal("baz"))
		})

		It("pulses with the given version", func() {
			randName := generateRandName()
			emitter.PulseCounter(
				randName,
				metric.WithPulseVersion(1, 2),
				metric.WithPulseInterval(time.Millisecond),
			)

			var e *v2.Envelope
			Eventually(receiver).Should(Receive(&e))
			Expect(e.Tags["metric_version"].GetText()).To(Equal("1.2"))
		})
	})
})

func generateRandName() string {
	return fmt.Sprintf("rand-name-%d", rand.Int63())
}
